package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"projeto-vendas/internal/models"
)

var testStorage *Storage
var testFilial models.Filial
var testProduct models.Product

// TestMain é uma função especial que é executada antes de todos os outros testes neste pacote.
func TestMain(m *testing.M) {
	loadTestEnv()

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		log.Fatal("DB_NAME não está definido no ambiente de teste.")
	}

	createTestDatabase(dbName)

	var err error
	testStorage, err = NewStorage()
	if err != nil {
		log.Fatalf("Falha ao conectar à base de dados de teste: %v", err)
	}

	setup(testStorage)
	exitCode := m.Run()
	cleanup(dbName)

	os.Exit(exitCode)
}

// NOVO: Função para encontrar a raiz do projeto e carregar o .env.test
func loadTestEnv() {
	// Obtém o diretório do ficheiro de teste atual
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Não foi possível encontrar o caminho do ficheiro de teste.")
	}
	dir := filepath.Dir(filename)

	// Sobe na árvore de diretórios até encontrar o go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break // Encontrou a raiz do projeto
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("Não foi possível encontrar a raiz do projeto (go.mod).")
		}
		dir = parent
	}

	// Carrega o ficheiro .env.test a partir da raiz do projeto
	envPath := filepath.Join(dir, ".env.test")
	if err := godotenv.Load(envPath); err != nil {
		log.Fatalf("Erro ao carregar o ficheiro .env.test de %s: %v", envPath, err)
	}
}

// createTestDatabase conecta-se ao servidor e cria a base de dados de teste.
func createTestDatabase(dbName string) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"))

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao servidor PostgreSQL: %v", err)
	}
	defer conn.Close(context.Background())

	_, _ = conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))

	_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		log.Fatalf("Falha ao criar a base de dados de teste: %v", err)
	}
	log.Printf("Base de dados '%s' criada com sucesso para os testes.", dbName)
}

func setup(s *Storage) {
	initSQLScript := `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";
		CREATE TABLE IF NOT EXISTS filiais (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), nome VARCHAR(150) UNIQUE NOT NULL, endereco TEXT, data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW());
		CREATE TABLE IF NOT EXISTS usuarios (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), filial_id UUID, nome VARCHAR(100) NOT NULL, email VARCHAR(100) UNIQUE NOT NULL, senha_hash VARCHAR(255) NOT NULL, cargo VARCHAR(20) NOT NULL, data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW(), CONSTRAINT fk_filial_usuario FOREIGN KEY(filial_id) REFERENCES filiais(id) ON DELETE SET NULL);
		CREATE TABLE IF NOT EXISTS produtos (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			nome VARCHAR(150) UNIQUE NOT NULL,
			descricao TEXT,
			codigo_barras VARCHAR(100) UNIQUE,
			codigo_cnae VARCHAR(15),
			preco_custo DECIMAL(10, 2) NOT NULL DEFAULT 0,
			percentual_lucro DECIMAL(5, 2) NOT NULL DEFAULT 0,
			imposto_estadual DECIMAL(5, 2) NOT NULL DEFAULT 0,
			imposto_federal DECIMAL(5, 2) NOT NULL DEFAULT 0,
			preco_sugerido DECIMAL(10, 2) NOT NULL,
			data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			data_atualizacao TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS estoque_filiais (produto_id UUID NOT NULL, filial_id UUID NOT NULL, quantidade INT NOT NULL, data_atualizacao TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (produto_id, filial_id), CONSTRAINT fk_produto_estoque FOREIGN KEY(produto_id) REFERENCES produtos(id) ON DELETE CASCADE, CONSTRAINT fk_filial_estoque FOREIGN KEY(filial_id) REFERENCES filiais(id) ON DELETE CASCADE);
		CREATE TABLE IF NOT EXISTS vendas (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), usuario_id UUID NOT NULL, filial_id UUID NOT NULL, total_venda DECIMAL(10, 2) NOT NULL, data_venda TIMESTAMPTZ NOT NULL DEFAULT NOW(), CONSTRAINT fk_usuario_venda FOREIGN KEY(usuario_id) REFERENCES usuarios(id) ON DELETE RESTRICT, CONSTRAINT fk_filial_venda FOREIGN KEY(filial_id) REFERENCES filiais(id) ON DELETE RESTRICT);
		CREATE TABLE IF NOT EXISTS itens_venda (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), venda_id UUID NOT NULL, produto_id UUID NOT NULL, quantidade INT NOT NULL, preco_unitario DECIMAL(10, 2) NOT NULL, CONSTRAINT fk_venda FOREIGN KEY(venda_id) REFERENCES vendas(id) ON DELETE CASCADE, CONSTRAINT fk_produto FOREIGN KEY(produto_id) REFERENCES produtos(id) ON DELETE RESTRICT);
	`
	_, err := s.Dbpool.Exec(context.Background(), initSQLScript)
	if err != nil {
		log.Fatalf("Falha ao executar o script de inicialização: %v", err)
	}

	testFilial = models.Filial{ID: uuid.New(), Nome: "Filial de Teste"}
	_, err = s.Dbpool.Exec(context.Background(), "INSERT INTO filiais (id, nome) VALUES ($1, $2)", testFilial.ID, testFilial.Nome)
	if err != nil {
		log.Fatalf("Falha ao inserir filial de teste: %v", err)
	}

	testProduct = models.Product{
		ID:              uuid.New(),
		Nome:            "Produto de Teste",
		CodigoBarras:    "123456789",
		PrecoCusto:      5.0,
		PercentualLucro: 50.0,
		ImpostoEstadual: 18.0,
		ImpostoFederal:  12.0,
		PrecoSugerido:   9.0, // 5 * (1 + 0.5 + 0.18 + 0.12) = 5 * 1.8 = 9.0
	}
	_, err = s.Dbpool.Exec(context.Background(), "INSERT INTO produtos (id, nome, codigo_barras, preco_custo, percentual_lucro, imposto_estadual, imposto_federal, preco_sugerido) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		testProduct.ID, testProduct.Nome, testProduct.CodigoBarras, testProduct.PrecoCusto, testProduct.PercentualLucro, testProduct.ImpostoEstadual, testProduct.ImpostoFederal, testProduct.PrecoSugerido)
	if err != nil {
		log.Fatalf("Falha ao inserir produto de teste: %v", err)
	}
}

func cleanup(dbName string) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"))

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao servidor PostgreSQL para limpeza: %v", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
	if err != nil {
		log.Fatalf("Falha ao apagar a base de dados de teste: %v", err)
	}
	log.Printf("Base de dados '%s' apagada com sucesso.", dbName)
}


// cleanup apaga a base de dados de teste.
func cleanup(dbName string) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"))
	
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao servidor PostgreSQL para limpeza: %v", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
	if err != nil {
		log.Fatalf("Falha ao apagar a base de dados de teste: %v", err)
	}
	log.Printf("Base de dados '%s' apagada com sucesso.", dbName)
}

// TestSearchProductsForSale testa a funcionalidade de busca de produtos.
func TestSearchProductsForSale(t *testing.T) {
	initialStock := 10
	_, err := testStorage.Dbpool.Exec(context.Background(), "INSERT INTO estoque_filiais (produto_id, filial_id, quantidade) VALUES ($1, $2, $3)", testProduct.ID, testFilial.ID, initialStock)
	if err != nil {
		t.Fatalf("Falha ao configurar o stock para o teste: %v", err)
	}

	t.Run("Deve encontrar o produto pelo nome", func(t *testing.T) {
		products, err := testStorage.SearchProductsForSale("Produto de Teste", testFilial.ID)
		if err != nil {
			t.Errorf("Erro inesperado ao buscar produto: %v", err)
		}
		if len(products) != 1 {
			t.Errorf("Esperava encontrar 1 produto, mas encontrou %d", len(products))
		}
		if len(products) > 0 && products[0].ID != testProduct.ID {
			t.Errorf("Produto encontrado (%s) não é o esperado (%s)", products[0].ID, testProduct.ID)
		}
	})

	t.Run("Deve encontrar o produto pelo código de barras", func(t *testing.T) {
		products, err := testStorage.SearchProductsForSale("123456789", testFilial.ID)
		if err != nil {
			t.Errorf("Erro inesperado ao buscar produto: %v", err)
		}
		if len(products) != 1 {
			t.Errorf("Esperava encontrar 1 produto, mas encontrou %d", len(products))
		}
	})

	t.Run("Não deve encontrar o produto numa filial diferente", func(t *testing.T) {
		wrongFilialID := uuid.New()
		products, err := testStorage.SearchProductsForSale("Produto de Teste", wrongFilialID)
		if err != nil {
			t.Errorf("Erro inesperado ao buscar produto: %v", err)
		}
		if len(products) > 0 {
			t.Errorf("Não deveria encontrar produtos, mas encontrou %d", len(products))
		}
	})
}

// TestRegisterSale testa a lógica de registo de venda e atualização de stock.
func TestRegisterSale(t *testing.T) {
	testUser := models.User{ID: uuid.New(), Nome: "Vendedor Teste", Email: "vendedor@teste.com", Cargo: "vendedor", SenhaHash: "hash"}
	_, err := testStorage.Dbpool.Exec(context.Background(), "INSERT INTO usuarios (id, nome, email, cargo, senha_hash, filial_id) VALUES ($1, $2, $3, $4, $5, $6)", testUser.ID, testUser.Nome, testUser.Email, testUser.Cargo, testUser.SenhaHash, testFilial.ID)
	if err != nil {
		t.Fatalf("Falha ao inserir utilizador de teste: %v", err)
	}

	t.Run("Deve registar a venda e dar baixa no stock com sucesso", func(t *testing.T) {
		_, err := testStorage.Dbpool.Exec(context.Background(), "UPDATE estoque_filiais SET quantidade = 10 WHERE produto_id = $1 AND filial_id = $2", testProduct.ID, testFilial.ID)
		if err != nil {
			t.Fatalf("Falha ao resetar o stock: %v", err)
		}

		saleItems := []models.ItemVenda{
			{ProdutoID: testProduct.ID, Quantidade: 3, PrecoUnitario: testProduct.PrecoSugerido},
		}
		sale := models.Venda{
			UsuarioID:  testUser.ID,
			FilialID:   testFilial.ID,
			TotalVenda: 3 * testProduct.PrecoSugerido,
		}

		err = testStorage.RegisterSale(sale, saleItems)
		if err != nil {
			t.Fatalf("Registo de venda falhou inesperadamente: %v", err)
		}

		var finalStock int
		err = testStorage.Dbpool.QueryRow(context.Background(), "SELECT quantidade FROM estoque_filiais WHERE produto_id = $1 AND filial_id = $2", testProduct.ID, testFilial.ID).Scan(&finalStock)
		if err != nil {
			t.Fatalf("Falha ao verificar o stock final: %v", err)
		}
		if finalStock != 7 {
			t.Errorf("Esperava que o stock final fosse 7, mas foi %d", finalStock)
		}
	})

	t.Run("Deve falhar ao tentar vender mais do que o stock disponível", func(t *testing.T) {
		saleItems := []models.ItemVenda{
			{ProdutoID: testProduct.ID, Quantidade: 8, PrecoUnitario: testProduct.PrecoSugerido}, // Stock atual é 7
		}
		sale := models.Venda{
			UsuarioID:  testUser.ID,
			FilialID:   testFilial.ID,
			TotalVenda: 8 * testProduct.PrecoSugerido,
		}

		err := testStorage.RegisterSale(sale, saleItems)
		if err == nil {
			t.Error("Esperava um erro de stock insuficiente, mas a venda foi registada com sucesso.")
		}
	})
}
