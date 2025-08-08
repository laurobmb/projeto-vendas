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

	"github.com/google/uuid"
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

// NOVO: Structs para Empresa e Sócio
type Empresa struct {
	ID           uuid.UUID
	RazaoSocial  string
	NomeFantasia string
	CNPJ         string
	Endereco     string
}

type Socio struct {
	EmpresaID uuid.UUID
	Nome      string
	Telefone  string
	Idade     int
	Email     string
	CPF       string
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

	dbpool := connectToDB()
	defer dbpool.Close()

	// Passo 1: Criar ou garantir que a empresa e os sócios existem
	empresaID, err := createOrGetEmpresa(dbpool)
	if err != nil {
		log.Fatalf("🚨 Erro ao criar/obter empresa: %v", err)
	}
	log.Println("✅ Garantida a existência dos dados da empresa.")

	err = createSocios(dbpool, empresaID)
	if err != nil {
		log.Fatalf("🚨 Erro ao criar sócios: %v", err)
	}
	log.Println("✅ Garantida a existência dos sócios.")

	// Passo 2: Criar ou garantir que as 3 filiais de teste existem
	filiais, err := createOrGetFiliais(dbpool)
	if err != nil {
		log.Fatalf("🚨 Erro ao criar/obter filiais: %v", err)
	}
	log.Printf("✅ Garantida a existência de %d filiais para popular o stock.", len(filiais))

	// Passo 3: Gerar a lista de produtos
	log.Println("⚙️ A gerar 2000 produtos variados...")
	produtos := generateProdutos(numProdutos)
	log.Println("✅ Lista de produtos gerada.")

	// Passo 4: Inserir os produtos no banco de dados
	log.Println("⏳ A inserir produtos no banco de dados... (Isto pode demorar um momento)")
	err = insertProdutos(dbpool, produtos)
	if err != nil {
		log.Fatalf("🚨 Falha ao inserir produtos: %v", err)
	}
	log.Println("✅ Produtos inseridos com sucesso na tabela 'produtos'.")

	// Passo 5: Popular o stock para cada produto nas 3 filiais
	log.Println("⏳ A popular a tabela 'estoque_filiais'...")
	err = populateEstoque(dbpool, produtos, filiais)
	if err != nil {
		log.Fatalf("🚨 Falha ao popular o stock: %v", err)
	}
	log.Println("✅ Stock populado com sucesso!")
	log.Println("🎉 Processo concluído!")
}

// NOVO: Cria/Atualiza o registo da empresa e retorna o seu ID.
func createOrGetEmpresa(dbpool *pgxpool.Pool) (uuid.UUID, error) {
	empresa := Empresa{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"), // ID Fixo
		RazaoSocial:  "Meu Atacarejo LTDA",
		NomeFantasia: "Atacarejo Preço Bom",
		CNPJ:         "12.345.678/0001-99",
		Endereco:     "Rua Principal, 123, Centro",
	}

	sql := `
		INSERT INTO empresa (id, razao_social, nome_fantasia, cnpj, endereco)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			razao_social = EXCLUDED.razao_social, nome_fantasia = EXCLUDED.nome_fantasia,
			cnpj = EXCLUDED.cnpj, endereco = EXCLUDED.endereco
	`
	_, err := dbpool.Exec(context.Background(), sql, empresa.ID, empresa.RazaoSocial, empresa.NomeFantasia, empresa.CNPJ, empresa.Endereco)
	return empresa.ID, err
}

// NOVO: Cria os sócios de exemplo para a empresa.
func createSocios(dbpool *pgxpool.Pool, empresaID uuid.UUID) error {
	socios := []Socio{
		{EmpresaID: empresaID, Nome: "Ana Silva", Telefone: "11 98765-4321", Idade: 45, Email: "ana.silva@email.com", CPF: "111.222.333-44"},
		{EmpresaID: empresaID, Nome: "Bruno Costa", Telefone: "21 91234-5678", Idade: 52, Email: "bruno.costa@email.com", CPF: "222.333.444-55"},
	}

	for _, socio := range socios {
		sql := `
			INSERT INTO socios (empresa_id, nome, telefone, idade, email, cpf)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (cpf) DO NOTHING
		`
		_, err := dbpool.Exec(context.Background(), sql, socio.EmpresaID, socio.Nome, socio.Telefone, socio.Idade, socio.Email, socio.CPF)
		if err != nil {
			return fmt.Errorf("falha ao inserir sócio %s: %w", socio.Nome, err)
		}
	}
	return nil
}

func createOrGetFiliais(dbpool *pgxpool.Pool) ([]Filial, error) {
	nomesFiliais := []string{"Filial Centro", "Filial Zona Sul", "Filial Leste"}
	for _, nome := range nomesFiliais {
		sql := `INSERT INTO filiais (nome) VALUES ($1) ON CONFLICT (nome) DO NOTHING`
		if _, err := dbpool.Exec(context.Background(), sql, nome); err != nil {
			return nil, fmt.Errorf("falha ao inserir filial %s: %w", nome, err)
		}
	}

	var filiais []Filial
	sqlBusca := `SELECT id, nome FROM filiais WHERE nome = ANY($1::text[])`
	rows, err := dbpool.Query(context.Background(), sqlBusca, nomesFiliais)
	if err != nil { return nil, err }
	defer rows.Close()

	for rows.Next() {
		var f Filial
		if err := rows.Scan(&f.ID, &f.Nome); err != nil { return nil, err }
		filiais = append(filiais, f)
	}

	if len(filiais) == 0 {
		return nil, errors.New("nenhuma filial foi encontrada ou criada")
	}
	return filiais, nil
}

// ... (resto do ficheiro sem alterações)
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
