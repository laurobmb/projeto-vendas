// Este ficheiro estaria localizado em: cmd/populando_banco/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid" // CORREÇÃO: Removido o ".com" extra.
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Estruturas para representar os nossos dados
type Produto struct {
	ID            uuid.UUID
	Nome          string
	Descricao     string
	CodigoBarras  string
	PrecoSugerido float64
}

type Filial struct {
	ID   uuid.UUID
	Nome string
}

const (
	numProdutos = 2000 // Total de produtos a serem criados
	numWorkers  = 10   // Número de goroutines para inserção no DB
)

// --- Listas de Dados para Geração Aleatória ---
var (
	categorias = map[string][]string{
		"Alimentos":    {"Arroz", "Feijão", "Macarrão", "Óleo de Soja", "Açúcar", "Café", "Leite em Pó", "Farinha de Trigo", "Biscoito", "Molho de Tomate"},
		"Limpeza":      {"Sabão em Pó", "Detergente", "Água Sanitária", "Desinfetante", "Amaciante de Roupas", "Limpador Multiuso"},
		"Higiene":      {"Sabonete", "Shampoo", "Condicionador", "Creme Dental", "Papel Higiénico", "Desodorante"},
		"Eletrónicos":  {"Smartphone", "Notebook", "TV LED 4K", "Fone de Ouvido Bluetooth", "Smartwatch", "Carregador Portátil", "Mouse Sem Fio"},
	}
	marcas  = []string{"Top", "Premium", "Value", "Gold", "Basic", "Tech", "Fresh", "Clean"}
	modelos = []string{"X100", "Pro", "S-Line", "Max", "Lite", "Ultra", "2.0"}
)

func main() {
	log.Println("🚀 Iniciando o programa para popular o banco de dados...")

	// Conecta ao banco de dados usando um pool de conexões
	dbpool := connectToDB()
	defer dbpool.Close()

	// Passo 1: Criar ou garantir que as 3 filiais de teste existem
	filiais, err := createOrGetFiliais(dbpool)
	if err != nil {
		log.Fatalf("🚨 Erro ao criar/obter filiais: %v", err)
	}
	log.Printf("✅ Garantida a existência de %d filiais para popular o stock.", len(filiais))

	// Passo 2: Gerar a lista de produtos
	log.Println("⚙️ A gerar 2000 produtos variados...")
	produtos := generateProdutos(numProdutos)
	log.Println("✅ Lista de produtos gerada.")

	// Passo 3: Inserir os produtos no banco de dados
	log.Println("⏳ A inserir produtos no banco de dados... (Isto pode demorar um momento)")
	err = insertProdutos(dbpool, produtos)
	if err != nil {
		log.Fatalf("🚨 Falha ao inserir produtos: %v", err)
	}
	log.Println("✅ Produtos inseridos com sucesso na tabela 'produtos'.")

	// Passo 4: Popular o stock para cada produto nas 3 filiais
	log.Println("⏳ A popular a tabela 'estoque_filiais'...")
	err = populateEstoque(dbpool, produtos, filiais)
	if err != nil {
		log.Fatalf("🚨 Falha ao popular o stock: %v", err)
	}
	log.Println("✅ Stock populado com sucesso!")
	log.Println("🎉 Processo concluído!")
}

// createOrGetFiliais insere as 3 filiais padrão se não existirem e retorna-as.
func createOrGetFiliais(dbpool *pgxpool.Pool) ([]Filial, error) {
	nomesFiliais := []string{"Filial Centro", "Filial Zona Sul", "Filial Leste"}
	
	// Insere as filiais. Se já existirem (pelo nome UNIQUE), não faz nada.
	for _, nome := range nomesFiliais {
		sql := `INSERT INTO filiais (nome) VALUES ($1) ON CONFLICT (nome) DO NOTHING`
		if _, err := dbpool.Exec(context.Background(), sql, nome); err != nil {
			return nil, fmt.Errorf("falha ao inserir filial %s: %w", nome, err)
		}
	}

	// Agora, busca os dados das 3 filiais para obter os seus IDs.
	var filiais []Filial
	sqlBusca := `SELECT id, nome FROM filiais WHERE nome = ANY($1::text[])`
	rows, err := dbpool.Query(context.Background(), sqlBusca, nomesFiliais)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var f Filial
		if err := rows.Scan(&f.ID, &f.Nome); err != nil {
			return nil, err
		}
		filiais = append(filiais, f)
	}

	if len(filiais) != 3 {
		return nil, errors.New("não foi possível criar ou encontrar as 3 filiais base")
	}

	return filiais, nil
}

// generateProdutos cria uma lista de produtos com dados aleatórios.
func generateProdutos(count int) []Produto {
	produtos := make([]Produto, count)
	for i := 0; i < count; i++ {
		catKeys := make([]string, 0, len(categorias))
		for k := range categorias {
			catKeys = append(catKeys, k)
		}
		categoriaNome := catKeys[rand.Intn(len(catKeys))]
		baseProduto := categorias[categoriaNome][rand.Intn(len(categorias[categoriaNome]))]
		marca := marcas[rand.Intn(len(marcas))]
		modelo := modelos[rand.Intn(len(modelos))]

		nomeCompleto := fmt.Sprintf("%s %s %s %d", baseProduto, marca, modelo, i)
		precoBase := 5.0 + rand.Float64()*100.0
		if categoriaNome == "Eletrónicos" {
			precoBase = 150.0 + rand.Float64()*4000.0
		}

		produtos[i] = Produto{
			ID:            uuid.New(),
			Nome:          nomeCompleto,
			Descricao:     fmt.Sprintf("Descrição para %s.", nomeCompleto),
			CodigoBarras:  strconv.FormatInt(time.Now().UnixNano()+int64(i), 10),
			PrecoSugerido: precoBase,
		}
	}
	return produtos
}

// insertProdutos insere uma lista de produtos no banco de dados usando concorrência.
func insertProdutos(dbpool *pgxpool.Pool, produtos []Produto) error {
	sqlStatement := `INSERT INTO produtos (id, nome, descricao, codigo_barras, preco_sugerido) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (nome) DO NOTHING`

	jobs := make(chan Produto, len(produtos))
	for _, p := range produtos {
		jobs <- p
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for produto := range jobs {
				_, err := dbpool.Exec(context.Background(), sqlStatement, produto.ID, produto.Nome, produto.Descricao, produto.CodigoBarras, produto.PrecoSugerido)
				if err != nil {
					log.Printf("Aviso: Falha ao inserir produto %s: %v", produto.Nome, err)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// populateEstoque insere registos de stock para cada produto em cada filial.
func populateEstoque(dbpool *pgxpool.Pool, produtos []Produto, filiais []Filial) error {
	sqlStatement := `INSERT INTO estoque_filiais (produto_id, filial_id, quantidade) VALUES ($1, $2, $3) ON CONFLICT (produto_id, filial_id) DO UPDATE SET quantidade = EXCLUDED.quantidade`

	type estoqueJob struct {
		produtoID uuid.UUID
		filialID  uuid.UUID
	}

	jobs := make(chan estoqueJob, len(produtos)*len(filiais))
	for _, p := range produtos {
		for _, f := range filiais {
			jobs <- estoqueJob{produtoID: p.ID, filialID: f.ID}
		}
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				quantidade := rand.Intn(450) + 50 // Stock entre 50 e 500
				_, err := dbpool.Exec(context.Background(), sqlStatement, job.produtoID, job.filialID, quantidade)
				if err != nil {
					log.Printf("Aviso: Falha ao inserir stock para produto %s na filial %s: %v", job.produtoID, job.filialID, err)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// connectToDB cria e retorna um pool de conexões com o PostgreSQL.
func connectToDB() *pgxpool.Pool {
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Ficheiro .env não encontrado. Usando variáveis de ambiente do sistema.")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
	)

	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados: %v\n", err)
	}

	err = dbpool.Ping(context.Background())
	if err != nil {
		log.Fatalf("Ping ao banco de dados falhou: %v\n", err)
	}

	return dbpool
}
