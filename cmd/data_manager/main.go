// Este ficheiro estaria localizado em: cmd/data_manager/main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
)

// O script SQL para inicializar o banco de dados com a estrutura multi-filial.
const initSQLScript = `
-- Habilita a extensão para gerar UUIDs, caso ainda não esteja habilitada.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tabela de Filiais (Supermercados/Lojas)
CREATE TABLE IF NOT EXISTS filiais (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome VARCHAR(150) UNIQUE NOT NULL,
    endereco TEXT,
    data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tabela de Usuários
CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filial_id UUID,
    nome VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    senha_hash VARCHAR(255) NOT NULL,
    cargo VARCHAR(20) NOT NULL CHECK (cargo IN ('vendedor', 'estoquista', 'admin')),
    data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_filial_usuario
        FOREIGN KEY(filial_id)
        REFERENCES filiais(id)
        ON DELETE SET NULL
);

-- Tabela de Produtos (Catálogo Mestre)
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

    preco_sugerido DECIMAL(10, 2) NOT NULL CHECK (preco_sugerido >= 0),
    data_criacao TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    data_atualizacao TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tabela de Estoque por Filial
CREATE TABLE IF NOT EXISTS estoque_filiais (
    produto_id UUID NOT NULL,
    filial_id UUID NOT NULL,
    quantidade INT NOT NULL CHECK (quantidade >= 0),
    data_atualizacao TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (produto_id, filial_id),
    CONSTRAINT fk_produto_estoque
        FOREIGN KEY(produto_id)
        REFERENCES produtos(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_filial_estoque
        FOREIGN KEY(filial_id)
        REFERENCES filiais(id)
        ON DELETE CASCADE
);

-- Tabela de Vendas
CREATE TABLE IF NOT EXISTS vendas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL,
    filial_id UUID NOT NULL,
    total_venda DECIMAL(10, 2) NOT NULL,
    data_venda TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_usuario_venda
        FOREIGN KEY(usuario_id)
        REFERENCES usuarios(id)
        ON DELETE RESTRICT,
    CONSTRAINT fk_filial_venda
        FOREIGN KEY(filial_id)
        REFERENCES filiais(id)
        ON DELETE RESTRICT
);

-- Tabela de Itens da Venda
CREATE TABLE IF NOT EXISTS itens_venda (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venda_id UUID NOT NULL,
    produto_id UUID NOT NULL,
    quantidade INT NOT NULL CHECK (quantidade > 0),
    preco_unitario DECIMAL(10, 2) NOT NULL,
    CONSTRAINT fk_venda
        FOREIGN KEY(venda_id)
        REFERENCES vendas(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_produto
        FOREIGN KEY(produto_id)
        REFERENCES produtos(id)
        ON DELETE RESTRICT
);
-- NOVAS TABELAS PARA DADOS DA EMPRESA --

-- Tabela da Empresa (desenhada para ter apenas um registo)
CREATE TABLE IF NOT EXISTS empresa (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    razao_social VARCHAR(255) NOT NULL,
    nome_fantasia VARCHAR(255),
    cnpj VARCHAR(18) UNIQUE NOT NULL,
    endereco TEXT
);

-- Tabela de Sócios, ligados à empresa
CREATE TABLE IF NOT EXISTS socios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL,
    nome VARCHAR(150) NOT NULL,
    telefone VARCHAR(20),
    idade INT,
    email VARCHAR(150),
    cpf VARCHAR(14) UNIQUE,
    CONSTRAINT fk_empresa_socio
        FOREIGN KEY(empresa_id)
        REFERENCES empresa(id)
        ON DELETE CASCADE
);

-- Índices para melhorar a performance
CREATE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios(email);
CREATE INDEX IF NOT EXISTS idx_estoque_filial_id ON estoque_filiais(filial_id);
CREATE INDEX IF NOT EXISTS idx_vendas_filial_id ON vendas(filial_id);
`

func main() {
	// --- Definição das Flags ---
	initDB := flag.Bool("init", false, "Inicializa o banco de dados e as tabelas.")
	createFilial := flag.String("filial", "", "Cria uma nova filial com o nome especificado.")
	filialEndereco := flag.String("endereco", "", "Endereço da nova filial (opcional, usado com -filial).")
	listFiliais := flag.Bool("list-filiais", false, "Lista todas as filiais existentes com os seus IDs.") // NOVA FLAG

	flag.Parse()

	// --- Lógica para decidir qual ação executar ---
	if *initDB {
		log.Println("Flag -init detectada. A iniciar a configuração do banco de dados...")
		runInitScript()
		log.Println("Configuração do banco de dados concluída.")
	} else if *createFilial != "" {
		log.Printf("Flag -filial detectada. A tentar criar a filial '%s'...", *createFilial)
		runCreateFilial(*createFilial, *filialEndereco)
	} else if *listFiliais {
		log.Println("Flag -list-filiais detectada. A listar as filiais...")
		runListFiliais()
	} else {
		log.Println("Nenhuma ação especificada. Use -init, -filial, ou -list-filiais.")
		flag.Usage()
	}
}

// NOVA FUNÇÃO: Lista todas as filiais.
func runListFiliais() {
	conn := connectToDB()
	defer conn.Close(context.Background())

	log.Println("A obter filiais do banco de dados...")

	rows, err := conn.Query(context.Background(), "SELECT id, nome, endereco FROM filiais ORDER BY nome")
	if err != nil {
		log.Fatalf("Falha ao obter filiais: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("\n--- Filiais Registadas ---")
	fmt.Println("------------------------------------------------------------------")
	fmt.Printf("%-38s | %-25s | %s\n", "ID", "NOME", "ENDEREÇO")
	fmt.Println("------------------------------------------------------------------")

	count := 0
	for rows.Next() {
		var id, nome string
		var enderecoPtr *string // Usar ponteiro para lidar com endereços que podem ser NULL

		if err := rows.Scan(&id, &nome, &enderecoPtr); err != nil {
			log.Fatalf("Falha ao ler a linha da filial: %v\n", err)
		}
		
		endereco := "N/A"
		if enderecoPtr != nil {
			endereco = *enderecoPtr
		}

		fmt.Printf("%-38s | %-25s | %s\n", id, nome, endereco)
		count++
	}
	fmt.Println("------------------------------------------------------------------")
	if count == 0 {
		fmt.Println("Nenhuma filial encontrada.")
	}
	fmt.Println()
}

func runCreateFilial(nome string, endereco string) {
	conn := connectToDB()
	defer conn.Close(context.Background())

	sqlStatement := `INSERT INTO filiais (nome, endereco) VALUES ($1, $2) RETURNING id`
	var newID string

	err := conn.QueryRow(context.Background(), sqlStatement, nome, endereco).Scan(&newID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Fatalf("Erro: A filial com o nome '%s' já existe.\n", nome)
		}
		log.Fatalf("Falha ao criar a filial: %v\n", err)
	}

	log.Printf("✅ Filial '%s' criada com sucesso! ID: %s\n", nome, newID)
}

func runInitScript() {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	maintenanceDSN := fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable", dbUser, dbPass, dbHost)
	maintenanceConn, err := pgx.Connect(context.Background(), maintenanceDSN)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados de manutenção 'postgres': %v\n", err)
	}
	defer maintenanceConn.Close(context.Background())

	var exists bool
	err = maintenanceConn.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		log.Fatalf("Falha ao verificar a existência do banco de dados: %v\n", err)
	}

	if !exists {
		log.Printf("Banco de dados '%s' não encontrado. A criar...", dbName)
		_, err = maintenanceConn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			log.Fatalf("Falha ao criar o banco de dados '%s': %v", dbName, err)
		}
		log.Printf("Banco de dados '%s' criado com sucesso.", dbName)
	} else {
		log.Printf("Banco de dados '%s' já existe.", dbName)
	}

	maintenanceConn.Close(context.Background())

	conn := connectToDB()
	defer conn.Close(context.Background())

	log.Printf("Conectado ao banco de dados '%s' com sucesso!", dbName)

	_, err = conn.Exec(context.Background(), initSQLScript)
	if err != nil {
		log.Fatalf("Falha ao executar o script de inicialização das tabelas: %v\n", err)
	}

	log.Println("Tabelas e dados iniciais criados/verificados com sucesso!")
}

func connectToDB() *pgx.Conn {
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

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados: %v\n", err)
	}
	return conn
}
