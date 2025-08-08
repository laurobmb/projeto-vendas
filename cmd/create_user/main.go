// Este arquivo seria localizado em: cmd/create_user/main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// --- Definição das Flags de Linha de Comando ---
	name := flag.String("name", "", "Nome completo do usuário (obrigatório)")
	email := flag.String("email", "", "Email de login do usuário (obrigatório)")
	password := flag.String("password", "", "Senha do usuário (obrigatório)")
	role := flag.String("role", "", "Cargo do usuário: 'admin', 'estoquista', ou 'vendedor' (obrigatório)")
	filialID := flag.String("filialid", "", "ID da filial à qual o usuário pertence (opcional)")

	flag.Parse()

	// --- Validação das Entradas ---
	if *name == "" || *email == "" || *password == "" || *role == "" {
		log.Println("Erro: As flags -name, -email, -password, e -role são obrigatórias.")
		flag.Usage()
		os.Exit(1)
	}

	switch *role {
	case "admin", "estoquista", "vendedor":
		// Cargo válido
	default:
		log.Fatalf("Erro: Cargo inválido '%s'. Use 'admin', 'estoquista', ou 'vendedor'.\n", *role)
	}

	// --- Conexão com o Banco de Dados ---
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis de ambiente do sistema.")
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
	defer conn.Close(context.Background())

	// --- Lógica de Criação do Usuário ---
	// Lida com o filial_id opcional e verifica sua existência.
	var filialIDValue interface{}
	if *filialID != "" {
		parsedUUID, err := uuid.Parse(*filialID)
		if err != nil {
			log.Fatalf("Erro: O ID da filial '%s' não é um UUID válido: %v\n", *filialID, err)
		}
		
		// CORREÇÃO: Verifica se a filial existe ANTES de tentar criar o usuário.
		var exists bool
		err = conn.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM filiais WHERE id = $1)", parsedUUID).Scan(&exists)
		if err != nil {
			log.Fatalf("Falha ao verificar a existência da filial: %v\n", err)
		}
		if !exists {
			log.Fatalf("Erro: A filial com o ID '%s' não foi encontrada. Por favor, crie a filial primeiro.", *filialID)
		}
		filialIDValue = parsedUUID
	} else {
		filialIDValue = nil
	}

	// Hashear a senha usando bcrypt.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Falha ao hashear a senha: %v\n", err)
	}

	// Prepara e executa o SQL para inserção.
	sqlStatement := `INSERT INTO usuarios (nome, email, senha_hash, cargo, filial_id) VALUES ($1, $2, $3, $4, $5)`
	_, err = conn.Exec(context.Background(), sqlStatement, *name, *email, string(hashedPassword), *role, filialIDValue)
	if err != nil {
		var pgErr *pgconn.PgError
		// Verifica se o erro é de violação de chave única (email já existe).
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Fatalf("Erro: Não foi possível criar o usuário. O email '%s' já está em uso.\n", *email)
		}
		// Para outros tipos de erro.
		log.Fatalf("Falha ao inserir usuário no banco de dados: %v\n", err)
	}

	log.Printf("✅ Usuário '%s' criado com sucesso!\n", *name)
}
