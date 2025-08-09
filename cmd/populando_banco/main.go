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
	"golang.org/x/crypto/bcrypt"
)

// Estruturas para representar os nossos dados
type Produto struct {
	ID              uuid.UUID
	Nome            string
	Descricao       string
	CodigoBarras    string
	CodigoCNAE      string // NOVO	
	PrecoCusto      float64
	PercentualLucro float64
	ImpostoEstadual float64
	ImpostoFederal  float64
	PrecoSugerido   float64
}

type Filial struct {
	ID   uuid.UUID
	Nome string
}

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

type User struct {
	ID       uuid.UUID
	Nome     string
	Email    string
	FilialID uuid.UUID
}

type StockItem struct {
	ProdutoID     uuid.UUID
	FilialID      uuid.UUID
	Quantidade    int
	PrecoSugerido float64
}

const (
	numProdutos = 2000 // Total de produtos a serem criados
	numWorkers  = 10   // Número de goroutines para inserção no DB
)

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

	// Passo 1: Empresa e Sócios
	empresaID, err := createOrGetEmpresa(dbpool)
	if err != nil { log.Fatalf("🚨 Erro ao criar/obter empresa: %v", err) }
	log.Println("✅ Garantida a existência dos dados da empresa.")
	err = createSocios(dbpool, empresaID)
	if err != nil { log.Fatalf("🚨 Erro ao criar sócios: %v", err) }
	log.Println("✅ Garantida a existência dos sócios.")

	// Passo 2: Filiais
	filiais, err := createOrGetFiliais(dbpool)
	if err != nil { log.Fatalf("🚨 Erro ao criar/obter filiais: %v", err) }
	log.Printf("✅ Garantida a existência de %d filiais.", len(filiais))

	// Passo 3: Utilizadores (Admin, Estoquistas e Vendedores)
	err = createAdmin(dbpool)
	if err != nil { log.Fatalf("🚨 Erro ao criar utilizador admin: %v", err) }
	log.Println("✅ Garantida a existência do utilizador admin.")

	err = createStockManagers(dbpool, filiais)
	if err != nil { log.Fatalf("🚨 Erro ao criar estoquistas: %v", err) }
	log.Println("✅ Garantida a existência dos estoquistas.")

	vendedores, err := createSellers(dbpool, filiais)
	if err != nil { log.Fatalf("🚨 Erro ao criar vendedores: %v", err) }
	log.Printf("✅ Garantida a existência de %d vendedores.", len(vendedores))

	// Passo 4: Produtos
	log.Println("⚙️ A gerar 2000 produtos variados...")
	produtos := generateProdutos(numProdutos)
	log.Println("✅ Lista de produtos gerada.")
	log.Println("⏳ A inserir produtos no banco de dados...")
	err = insertProdutos(dbpool, produtos)
	if err != nil { log.Fatalf("🚨 Falha ao inserir produtos: %v", err) }
	log.Println("✅ Produtos inseridos com sucesso.")

	// Passo 5: Stock
	log.Println("⏳ A popular a tabela 'estoque_filiais'...")
	err = populateEstoque(dbpool, produtos, filiais)
	if err != nil { log.Fatalf("🚨 Falha ao popular o stock: %v", err) }
	log.Println("✅ Stock populado com sucesso!")

	// Passo 6: Vendas
	log.Println("⏳ A simular vendas para 1/3 do stock...")
	err = createSales(dbpool, vendedores)
	if err != nil { log.Fatalf("🚨 Falha ao simular vendas: %v", err) }
	log.Println("✅ Vendas simuladas com sucesso!")
	
	log.Println("🎉 Processo concluído!")
}

func createAdmin(dbpool *pgxpool.Pool) error {
	adminData := map[string]string{
		"name": "Admin Geral", "email": "admin@email.com", "password": "1q2w3e",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminData["password"]), bcrypt.DefaultCost)
	if err != nil { return err }

	sql := `
		INSERT INTO usuarios (nome, email, senha_hash, cargo, filial_id)
		VALUES ($1, $2, $3, 'admin', NULL)
		ON CONFLICT (email) DO UPDATE SET nome = EXCLUDED.nome, cargo = 'admin'
	`
	_, err = dbpool.Exec(context.Background(), sql, adminData["name"], adminData["email"], string(hashedPassword))
	if err != nil {
		return fmt.Errorf("falha ao inserir utilizador admin: %w", err)
	}
	return nil
}

func createStockManagers(dbpool *pgxpool.Pool, filiais []Filial) error {
	if len(filiais) < 3 {
		return errors.New("são necessárias pelo menos 3 filiais para criar os estoquistas")
	}

	stockManagersData := []map[string]string{
		{"name": "Carlos Estoquista", "email": "carlos.estoque@email.com", "password": "1q2w3e"},
		{"name": "Mariana Estoquista", "email": "mariana.estoque@email.com", "password": "1q2w3e"},
		{"name": "Ricardo Estoquista", "email": "ricardo.estoque@email.com", "password": "1q2w3e"},
	}

	for i, data := range stockManagersData {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data["password"]), bcrypt.DefaultCost)
		if err != nil { return err }

		filial := filiais[i]
		
		sql := `
			INSERT INTO usuarios (nome, email, senha_hash, cargo, filial_id)
			VALUES ($1, $2, $3, 'estoquista', $4)
			ON CONFLICT (email) DO UPDATE SET nome = EXCLUDED.nome
		`
		_, err = dbpool.Exec(context.Background(), sql, data["name"], data["email"], string(hashedPassword), filial.ID)
		if err != nil {
			return fmt.Errorf("falha ao inserir estoquista %s: %w", data["name"], err)
		}
	}
	return nil
}


func createSellers(dbpool *pgxpool.Pool, filiais []Filial) ([]User, error) {
	sellersData := []map[string]string{
		{"name": "João Vendedor", "email": "joao.vendas@email.com", "password": "1q2w3e"},
		{"name": "Pedro Vendedor", "email": "pedro.vendas@email.com", "password": "1q2w3e"},
		{"name": "Camila Vendedora", "email": "camila.vendas@email.com", "password": "1q2w3e"},
	}

	var createdSellers []User
	for _, data := range sellersData {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data["password"]), bcrypt.DefaultCost)
		if err != nil { return nil, err }

		filialAleatoria := filiais[rand.Intn(len(filiais))]
		
		var userID uuid.UUID
		sql := `
			INSERT INTO usuarios (nome, email, senha_hash, cargo, filial_id)
			VALUES ($1, $2, $3, 'vendedor', $4)
			ON CONFLICT (email) DO UPDATE SET nome = EXCLUDED.nome
			RETURNING id
		`
		err = dbpool.QueryRow(context.Background(), sql, data["name"], data["email"], string(hashedPassword), filialAleatoria.ID).Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("falha ao inserir vendedor %s: %w", data["name"], err)
		}
		createdSellers = append(createdSellers, User{ID: userID, Nome: data["name"], Email: data["email"], FilialID: filialAleatoria.ID})
	}
	return createdSellers, nil
}

func createSales(dbpool *pgxpool.Pool, vendedores []User) error {
	var stockItems []StockItem
	sqlItems := `SELECT p.id, ef.filial_id, ef.quantidade, p.preco_sugerido FROM estoque_filiais ef JOIN produtos p ON p.id = ef.produto_id WHERE ef.quantidade > 10`
	rows, err := dbpool.Query(context.Background(), sqlItems)
	if err != nil { return err }
	defer rows.Close()
	for rows.Next() {
		var item StockItem
		if err := rows.Scan(&item.ProdutoID, &item.FilialID, &item.Quantidade, &item.PrecoSugerido); err != nil { return err }
		stockItems = append(stockItems, item)
	}

	numItemsToSell := len(stockItems) / 3
	rand.Shuffle(len(stockItems), func(i, j int) { stockItems[i], stockItems[j] = stockItems[j], stockItems[i] })
	itemsToSell := stockItems[:numItemsToSell]

	for _, item := range itemsToSell {
		vendedorDaFilial := vendedores[0]
		for _, v := range vendedores {
			if v.FilialID == item.FilialID {
				vendedorDaFilial = v
				break
			}
		}

		quantidadeVenda := rand.Intn(item.Quantidade/3) + 1
		totalVenda := float64(quantidadeVenda) * item.PrecoSugerido

		tx, err := dbpool.Begin(context.Background())
		if err != nil { continue }

		var vendaID uuid.UUID
		sqlVenda := `INSERT INTO vendas (usuario_id, filial_id, total_venda) VALUES ($1, $2, $3) RETURNING id`
		err = tx.QueryRow(context.Background(), sqlVenda, vendedorDaFilial.ID, item.FilialID, totalVenda).Scan(&vendaID)
		if err != nil { tx.Rollback(context.Background()); continue }

		sqlItemVenda := `INSERT INTO itens_venda (venda_id, produto_id, quantidade, preco_unitario) VALUES ($1, $2, $3, $4)`
		_, err = tx.Exec(context.Background(), sqlItemVenda, vendaID, item.ProdutoID, quantidadeVenda, item.PrecoSugerido)
		if err != nil { tx.Rollback(context.Background()); continue }

		sqlStock := `UPDATE estoque_filiais SET quantidade = quantidade - $1 WHERE produto_id = $2 AND filial_id = $3`
		_, err = tx.Exec(context.Background(), sqlStock, quantidadeVenda, item.ProdutoID, item.FilialID)
		if err != nil { tx.Rollback(context.Background()); continue }

		tx.Commit(context.Background())
	}
	return nil
}

func createOrGetEmpresa(dbpool *pgxpool.Pool) (uuid.UUID, error) {
	empresa := Empresa{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
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

// ATUALIZADO: Gera produtos com a nova estrutura de preços.
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
		
		custo := 5.0 + rand.Float64()*80.0
		if categoriaNome == "Eletrónicos" {
			custo = 100.0 + rand.Float64()*2500.0
		}

		lucro := 15.0 + rand.Float64()*35.0 // Lucro entre 15% e 50%
		impostoEst := 7.0 + rand.Float64()*11.0 // Imposto estadual entre 7% e 18%
		impostoFed := 5.0 + rand.Float64()*7.0  // Imposto federal entre 5% e 12%

		precoSugerido := custo * (1 + lucro/100 + impostoEst/100 + impostoFed/100)

		cnaeAleatorio := rand.Intn(10000000)
		
		produtos[i] = Produto{
			ID:              uuid.New(),
			Nome:            nomeCompleto,
			Descricao:       fmt.Sprintf("Descrição para %s.", nomeCompleto),
			CodigoBarras:    strconv.FormatInt(time.Now().UnixNano()+int64(i), 10),
			CodigoCNAE:      strconv.Itoa(cnaeAleatorio), // NOVO			
			PrecoCusto:      custo,
			PercentualLucro: lucro,
			ImpostoEstadual: impostoEst,
			ImpostoFederal:  impostoFed,
			PrecoSugerido:   precoSugerido,
		}
	}
	return produtos
}

// ATUALIZADO: Insere produtos com os novos campos de preço.
func insertProdutos(dbpool *pgxpool.Pool, produtos []Produto) error {
	sqlStatement := `
		INSERT INTO produtos (id, nome, descricao, codigo_barras, codigo_cnae, preco_custo, percentual_lucro, imposto_estadual, imposto_federal, preco_sugerido) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) 
		ON CONFLICT (nome) DO NOTHING
	`
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
				_, err := dbpool.Exec(context.Background(), sqlStatement, 
					produto.ID, produto.Nome, produto.Descricao, produto.CodigoBarras, produto.CodigoCNAE,  
					produto.PrecoCusto, produto.PercentualLucro, produto.ImpostoEstadual, produto.ImpostoFederal, produto.PrecoSugerido)
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
		log.Fatalf("Não foi possível conectar ao banco de dados: %v", err)
	}
	err = dbpool.Ping(context.Background())
	if err != nil {
		log.Fatalf("Ping ao banco de dados falhou: %v", err)
	}
	return dbpool
}
