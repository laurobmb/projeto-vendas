package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"projeto-vendas/internal/models"
)

// CORREÇÃO: A interface Store foi atualizada para incluir todas as funções necessárias.
type Store interface {
	GetUserByEmail(email string) (*models.User, error)
	GetFilialByID(id string) (*models.Filial, error)
	CountUsers() (int, error)
	GetUsersPaginated(limit, offset int) ([]models.User, error)
	CountProducts(searchQuery string) (int, error)
	GetProductsPaginatedAndFiltered(searchQuery string, limit, offset int) ([]models.Product, error)
	GetAllFiliais() ([]models.Filial, error)
	UpdateUser(userID string, user models.User, newPassword string) error
	CountSales(filialID string) (int, error)
	GetSalesPaginated(filialID string, limit, offset int) ([]models.SaleReportItem, error)
	SearchProductsForSale(query string, filialID uuid.UUID) ([]models.Product, error)
	RegisterSale(sale models.Venda, items []models.ItemVenda) error
	CreateProductWithInitialStock(product models.Product, filialID string, quantity int) error
	AddStockItem(productID, filialID string, quantity int) error
	GetAllProductsSimple() ([]models.Product, error)
	CountStockItems(filialID, searchQuery string) (int, error)
	GetStockItemsPaginated(filialID, searchQuery string, limit, offset int) ([]models.StockViewItem, error)
	UpdateStockQuantity(productID, filialID string, newQuantity int) error
	AddUser(user models.User, password string) error
	AddProduct(product models.Product) error
	UpdateSocio(socioID string, socio models.Socio) error
	GetEmpresa() (*models.Empresa, error)
	UpsertEmpresa(empresa models.Empresa) error
	GetSocios(empresaID uuid.UUID) ([]models.Socio, error)
	AddSocio(socio models.Socio) error
	DeleteSocioByID(id string) error
	DeleteUserByID(id string) error
	DeleteProductByID(id string) error
	GetProductStockByFilial(productID string) ([]models.StockDetail, error)
	AdjustStockQuantity(productID, filialID string, quantityToRemove int) error
	GetSalesSummary() ([]models.SalesSummary, error) // Função adicionada
}

type Storage struct {
	Dbpool *pgxpool.Pool
}

// NOVO: Struct para o resumo de vendas.
type SalesSummary struct {
	FilialNome  string  `json:"filial_nome"`
	TotalVendas float64 `json:"total_vendas"`
}

// CORREÇÃO: A função agora usa 'models.SalesSummary' para corresponder à interface.
func (s *Storage) GetSalesSummary() ([]models.SalesSummary, error) {
	var summary []models.SalesSummary
	sql := `
		SELECT f.nome, COALESCE(SUM(v.total_venda), 0) as total
		FROM filiais f
		LEFT JOIN vendas v ON f.id = v.filial_id
		GROUP BY f.nome
		ORDER BY total DESC
	`
	rows, err := s.Dbpool.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SalesSummary
		if err := rows.Scan(&item.FilialNome, &item.TotalVendas); err != nil {
			return nil, err
		}
		summary = append(summary, item)
	}
	return summary, nil
}

func NewStorage() (*Storage, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: Ficheiro .env não encontrado.")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("não foi possível conectar ao banco de dados: %w", err)
	}
	return &Storage{Dbpool: pool}, nil
}

// ... (todas as outras funções do ficheiro storage.go permanecem aqui, sem alterações)


func (s *Storage) UpdateSocio(socioID string, socio models.Socio) error {
	sql := `
		UPDATE socios 
		SET nome = $1, telefone = $2, idade = $3, email = $4, cpf = $5
		WHERE id = $6
	`
	cmdTag, err := s.Dbpool.Exec(context.Background(), sql, socio.Nome, socio.Telefone, socio.Idade, socio.Email, socio.CPF, socioID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("nenhum sócio foi atualizado (ID não encontrado?)")
	}
	return nil
}

func (s *Storage) CountSales(filialID string) (int, error) {
	var count int
	sql := `SELECT COUNT(*) FROM vendas`
	var args []interface{}
	if filialID != "" {
		sql += " WHERE filial_id = $1"
		args = append(args, filialID)
	}
	err := s.Dbpool.QueryRow(context.Background(), sql, args...).Scan(&count)
	return count, err
}

func (s *Storage) GetSalesPaginated(filialID string, limit, offset int) ([]models.SaleReportItem, error) {
	var sales []models.SaleReportItem
	sql := `
		SELECT v.id, v.data_venda, f.nome, u.nome, v.total_venda
		FROM vendas v
		JOIN filiais f ON v.filial_id = f.id
		JOIN usuarios u ON v.usuario_id = u.id
	`
	args := []interface{}{limit, offset}
	argCount := 2
	if filialID != "" {
		argCount++
		sql += fmt.Sprintf(" WHERE v.filial_id = $%d", argCount)
		args = append(args, filialID)
	}
	sql += " ORDER BY v.data_venda DESC LIMIT $1 OFFSET $2"
	rows, err := s.Dbpool.Query(context.Background(), sql, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var item models.SaleReportItem
		if err := rows.Scan(&item.VendaID, &item.DataVenda, &item.FilialNome, &item.VendedorNome, &item.TotalVenda); err != nil {
			return nil, err
		}
		sales = append(sales, item)
	}
	return sales, nil
}

func (s *Storage) SearchProductsForSale(query string, filialID uuid.UUID) ([]models.Product, error) {
	var products []models.Product
	sql := `
		SELECT p.id, p.nome, p.codigo_barras, p.preco_sugerido
		FROM produtos p
		JOIN estoque_filiais ef ON p.id = ef.produto_id
		WHERE ef.filial_id = $1 AND ef.quantidade > 0 AND (p.nome ILIKE $2 OR p.codigo_barras = $3)
		LIMIT 10
	`
	rows, err := s.Dbpool.Query(context.Background(), sql, filialID, "%"+query+"%", query)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Nome, &p.CodigoBarras, &p.PrecoSugerido); err != nil { return nil, err }
		products = append(products, p)
	}
	return products, nil
}

func (s *Storage) RegisterSale(sale models.Venda, items []models.ItemVenda) error {
	tx, err := s.Dbpool.Begin(context.Background())
	if err != nil { return fmt.Errorf("erro ao iniciar transação: %w", err) }
	defer tx.Rollback(context.Background())
	var vendaID uuid.UUID
	sqlVenda := `INSERT INTO vendas (usuario_id, filial_id, total_venda) VALUES ($1, $2, $3) RETURNING id`
	err = tx.QueryRow(context.Background(), sqlVenda, sale.UsuarioID, sale.FilialID, sale.TotalVenda).Scan(&vendaID)
	if err != nil { return fmt.Errorf("erro ao inserir venda: %w", err) }
	for _, item := range items {
		sqlItem := `INSERT INTO itens_venda (venda_id, produto_id, quantidade, preco_unitario) VALUES ($1, $2, $3, $4)`
		_, err := tx.Exec(context.Background(), sqlItem, vendaID, item.ProdutoID, item.Quantidade, item.PrecoUnitario)
		if err != nil { return fmt.Errorf("erro ao inserir item %s: %w", item.ProdutoID, err) }
		sqlStock := `
			UPDATE estoque_filiais SET quantidade = quantidade - $1
			WHERE produto_id = $2 AND filial_id = $3 AND quantidade >= $1
		`
		cmdTag, err := tx.Exec(context.Background(), sqlStock, item.Quantidade, item.ProdutoID, sale.FilialID)
		if err != nil { return fmt.Errorf("erro ao dar baixa no stock para o item %s: %w", item.ProdutoID, err) }
		if cmdTag.RowsAffected() == 0 {
			return fmt.Errorf("stock insuficiente para o produto %s na filial %s", item.ProdutoID, sale.FilialID)
		}
	}
	return tx.Commit(context.Background())
}

func (s *Storage) GetFilialByID(id string) (*models.Filial, error) {
	var filial models.Filial
	sql := `SELECT id, nome FROM filiais WHERE id = $1`
	err := s.Dbpool.QueryRow(context.Background(), sql, id).Scan(&filial.ID, &filial.Nome)
	return &filial, err
}

func (s *Storage) CreateProductWithInitialStock(product models.Product, filialID string, quantity int) error {
	var tx pgx.Tx
	tx, err := s.Dbpool.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("não foi possível iniciar a transação: %w", err)
	}
	defer tx.Rollback(context.Background())
	var newProductID string
	sqlProduct := `INSERT INTO produtos (nome, descricao, codigo_barras, preco_sugerido) VALUES ($1, $2, $3, $4) RETURNING id`
	err = tx.QueryRow(context.Background(), sqlProduct, product.Nome, product.Descricao, product.CodigoBarras, product.PrecoSugerido).Scan(&newProductID)
	if err != nil {
		return fmt.Errorf("falha ao inserir o produto na transação: %w", err)
	}
	sqlStock := `INSERT INTO estoque_filiais (produto_id, filial_id, quantidade) VALUES ($1, $2, $3)`
	_, err = tx.Exec(context.Background(), sqlStock, newProductID, filialID, quantity)
	if err != nil {
		return fmt.Errorf("falha ao inserir o stock na transação: %w", err)
	}
	return tx.Commit(context.Background())
}

func (s *Storage) AddStockItem(productID, filialID string, quantity int) error {
	sql := `
		INSERT INTO estoque_filiais (produto_id, filial_id, quantidade)
		VALUES ($1, $2, $3)
		ON CONFLICT (produto_id, filial_id)
		DO UPDATE SET quantidade = estoque_filiais.quantidade + EXCLUDED.quantidade;
	`
	_, err := s.Dbpool.Exec(context.Background(), sql, productID, filialID, quantity)
	return err
}

func (s *Storage) GetAllProductsSimple() ([]models.Product, error) {
	var products []models.Product
	sql := `SELECT id, nome FROM produtos ORDER BY nome`
	rows, err := s.Dbpool.Query(context.Background(), sql)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Nome); err != nil { return nil, err }
		products = append(products, p)
	}
	return products, nil
}

func (s *Storage) CountStockItems(filialID, searchQuery string) (int, error) {
	var count int
	sql := `SELECT COUNT(*) FROM estoque_filiais ef JOIN produtos p ON ef.produto_id = p.id`
	var args []interface{}
	whereClauses := ""
	if filialID != "" {
		whereClauses += " WHERE ef.filial_id = $1"
		args = append(args, filialID)
	}
	if searchQuery != "" {
		if len(args) > 0 {
			whereClauses += " AND"
		} else {
			whereClauses += " WHERE"
		}
		whereClauses += fmt.Sprintf(" (p.nome ILIKE $%d OR p.codigo_barras = $%d)", len(args)+1, len(args)+2)
		args = append(args, "%"+searchQuery+"%", searchQuery)
	}
	err := s.Dbpool.QueryRow(context.Background(), sql+whereClauses, args...).Scan(&count)
	return count, err
}

func (s *Storage) GetStockItemsPaginated(filialID, searchQuery string, limit, offset int) ([]models.StockViewItem, error) {
	var items []models.StockViewItem
	sql := `
		SELECT p.id, p.nome, p.codigo_barras, f.id, f.nome, ef.quantidade
		FROM estoque_filiais ef
		JOIN produtos p ON ef.produto_id = p.id
		JOIN filiais f ON ef.filial_id = f.id
	`
	args := []interface{}{limit, offset}
	argCount := 2
	whereClauses := ""
	if filialID != "" {
		argCount++
		whereClauses += fmt.Sprintf(" WHERE ef.filial_id = $%d", argCount)
		args = append(args, filialID)
	}
	if searchQuery != "" {
		if len(whereClauses) > 0 {
			whereClauses += " AND"
		} else {
			whereClauses += " WHERE"
		}
		whereClauses += fmt.Sprintf(" (p.nome ILIKE $%d OR p.codigo_barras = $%d)", argCount+1, argCount+2)
		args = append(args, "%"+searchQuery+"%", searchQuery)
	}
	sql += whereClauses + " ORDER BY p.nome, f.nome LIMIT $1 OFFSET $2"
	rows, err := s.Dbpool.Query(context.Background(), sql, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var item models.StockViewItem
		if err := rows.Scan(&item.ProdutoID, &item.ProdutoNome, &item.CodigoBarras, &item.FilialID, &item.FilialNome, &item.Quantidade); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Storage) UpdateStockQuantity(productID, filialID string, newQuantity int) error {
	sql := `
		UPDATE estoque_filiais SET quantidade = $1 
		WHERE produto_id = $2 AND filial_id = $3
	`
	cmdTag, err := s.Dbpool.Exec(context.Background(), sql, newQuantity, productID, filialID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("nenhum registo de stock foi atualizado (produto/filial não encontrado?)")
	}
	return nil
}

func (s *Storage) GetProductsPaginatedAndFiltered(searchQuery string, limit, offset int) ([]models.Product, error) {
	var products []models.Product
	sql := `
		SELECT p.id, p.nome, p.descricao, p.codigo_barras, p.preco_sugerido, 
			COALESCE(SUM(ef.quantidade), 0) as total_estoque,
			(COALESCE(SUM(ef.quantidade), 0) * p.preco_sugerido) as valor_total_estoque
		FROM produtos p
		LEFT JOIN estoque_filiais ef ON p.id = ef.produto_id
	`
	args := []interface{}{limit, offset}
	argCount := 2
	if searchQuery != "" {
		argCount++
		sql += fmt.Sprintf(" WHERE p.nome ILIKE $%d OR p.codigo_barras ILIKE $%d", argCount, argCount)
		args = append(args, "%"+searchQuery+"%")
	}
	sql += fmt.Sprintf(" GROUP BY p.id ORDER BY p.nome LIMIT $%d OFFSET $%d", 1, 2)
	
	rows, err := s.Dbpool.Query(context.Background(), sql, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Nome, &p.Descricao, &p.CodigoBarras, &p.PrecoSugerido, &p.TotalEstoque, &p.ValorTotalEstoque); err != nil { return nil, err }
		products = append(products, p)
	}
	return products, nil
}

func (s *Storage) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	sql := `SELECT id, nome, email, senha_hash, cargo, filial_id FROM usuarios WHERE email = $1`
	err := s.Dbpool.QueryRow(context.Background(), sql, email).Scan(&user.ID, &user.Nome, &user.Email, &user.SenhaHash, &user.Cargo, &user.FilialID)
	return &user, err
}

func (s *Storage) CountUsers() (int, error) {
	var count int
	err := s.Dbpool.QueryRow(context.Background(), "SELECT COUNT(*) FROM usuarios").Scan(&count)
	return count, err
}

func (s *Storage) GetUsersPaginated(limit, offset int) ([]models.User, error) {
	var users []models.User
	sql := `SELECT id, nome, email, cargo, filial_id FROM usuarios ORDER BY nome LIMIT $1 OFFSET $2`
	rows, err := s.Dbpool.Query(context.Background(), sql, limit, offset)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Nome, &u.Email, &u.Cargo, &u.FilialID); err != nil { return nil, err }
		users = append(users, u)
	}
	return users, nil
}

func (s *Storage) CountProducts(searchQuery string) (int, error) {
	var count int
	sql := "SELECT COUNT(*) FROM produtos"
	var err error
	if searchQuery != "" {
		sql += " WHERE nome ILIKE $1 OR codigo_barras ILIKE $1"
		err = s.Dbpool.QueryRow(context.Background(), sql, "%"+searchQuery+"%").Scan(&count)
	} else {
		err = s.Dbpool.QueryRow(context.Background(), sql).Scan(&count)
	}
	return count, err
}

func (s *Storage) GetAllFiliais() ([]models.Filial, error) {
	var filiais []models.Filial
	sql := `SELECT id, nome FROM filiais ORDER BY nome`
	rows, err := s.Dbpool.Query(context.Background(), sql)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var f models.Filial
		if err := rows.Scan(&f.ID, &f.Nome); err != nil { return nil, err }
		filiais = append(filiais, f)
	}
	return filiais, nil
}

func (s *Storage) DeleteUserByID(id string) error {
	_, err := s.Dbpool.Exec(context.Background(), "DELETE FROM usuarios WHERE id = $1", id)
	return err
}

func (s *Storage) DeleteProductByID(id string) error {
	_, err := s.Dbpool.Exec(context.Background(), "DELETE FROM produtos WHERE id = $1", id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("não é possível apagar o produto, pois ele está associado a vendas existentes")
		}
	}
	return err
}

func (s *Storage) GetProductStockByFilial(productID string) ([]models.StockDetail, error) {
	var details []models.StockDetail
	sql := `
		SELECT f.id, f.nome, ef.quantidade
		FROM estoque_filiais ef
		JOIN filiais f ON ef.filial_id = f.id
		WHERE ef.produto_id = $1
		ORDER BY f.nome
	`
	rows, err := s.Dbpool.Query(context.Background(), sql, productID)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var d models.StockDetail
		if err := rows.Scan(&d.FilialID, &d.FilialNome, &d.Quantidade); err != nil { return nil, err }
		details = append(details, d)
	}
	return details, nil
}

func (s *Storage) AdjustStockQuantity(productID, filialID string, quantityToRemove int) error {
	sql := `
		UPDATE estoque_filiais
		SET quantidade = quantidade - $1
		WHERE produto_id = $2 AND filial_id = $3 AND quantidade >= $1
	`
	cmdTag, err := s.Dbpool.Exec(context.Background(), sql, quantityToRemove, productID, filialID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("stock insuficiente para a baixa ou item/filial não encontrado")
	}
	return nil
}

func (s *Storage) AddUser(user models.User, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("erro ao gerar hash da senha: %w", err)
	}
	sql := `INSERT INTO usuarios (nome, email, senha_hash, cargo, filial_id) VALUES ($1, $2, $3, $4, $5)`
	_, err = s.Dbpool.Exec(context.Background(), sql, user.Nome, user.Email, string(hashedPassword), user.Cargo, user.FilialID)
	return err
}

func (s *Storage) AddProduct(product models.Product) error {
	sql := `INSERT INTO produtos (nome, descricao, codigo_barras, preco_sugerido) VALUES ($1, $2, $3, $4)`
	_, err := s.Dbpool.Exec(context.Background(), sql, product.Nome, product.Descricao, product.CodigoBarras, product.PrecoSugerido)
	return err
}

func (s *Storage) UpdateUser(userID string, user models.User, newPassword string) error {
	tx, err := s.Dbpool.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("não foi possível iniciar a transação: %w", err)
	}
	defer tx.Rollback(context.Background())

	sqlDetails := `UPDATE usuarios SET nome = $1, email = $2, cargo = $3, filial_id = $4 WHERE id = $5`
	_, err = tx.Exec(context.Background(), sqlDetails, user.Nome, user.Email, user.Cargo, user.FilialID, userID)
	if err != nil {
		return fmt.Errorf("falha ao atualizar detalhes do utilizador: %w", err)
	}

	if newPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("falha ao gerar hash da nova senha: %w", err)
		}
		sqlPass := `UPDATE usuarios SET senha_hash = $1 WHERE id = $2`
		_, err = tx.Exec(context.Background(), sqlPass, string(hashedPassword), userID)
		if err != nil {
			return fmt.Errorf("falha ao atualizar a senha do utilizador: %w", err)
		}
	}

	return tx.Commit(context.Background())
}

func (s *Storage) GetEmpresa() (*models.Empresa, error) {
	var empresa models.Empresa
	sql := `SELECT id, razao_social, nome_fantasia, cnpj, endereco FROM empresa LIMIT 1`
	err := s.Dbpool.QueryRow(context.Background(), sql).Scan(&empresa.ID, &empresa.RazaoSocial, &empresa.NomeFantasia, &empresa.CNPJ, &empresa.Endereco)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.Empresa{}, nil
		}
		return nil, err
	}
	return &empresa, nil
}

func (s *Storage) UpsertEmpresa(empresa models.Empresa) error {
	const fixedEmpresaID = "00000000-0000-0000-0000-000000000001"
	
	sql := `
		INSERT INTO empresa (id, razao_social, nome_fantasia, cnpj, endereco)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			razao_social = EXCLUDED.razao_social,
			nome_fantasia = EXCLUDED.nome_fantasia,
			cnpj = EXCLUDED.cnpj,
			endereco = EXCLUDED.endereco
	`
	_, err := s.Dbpool.Exec(context.Background(), sql, fixedEmpresaID, empresa.RazaoSocial, empresa.NomeFantasia, empresa.CNPJ, empresa.Endereco)
	return err
}

func (s *Storage) GetSocios(empresaID uuid.UUID) ([]models.Socio, error) {
	var socios []models.Socio
	if empresaID == uuid.Nil {
		return socios, nil
	}
	sql := `SELECT id, nome, telefone, idade, email, cpf FROM socios WHERE empresa_id = $1 ORDER BY nome`
	rows, err := s.Dbpool.Query(context.Background(), sql, empresaID)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var socio models.Socio
		if err := rows.Scan(&socio.ID, &socio.Nome, &socio.Telefone, &socio.Idade, &socio.Email, &socio.CPF); err != nil { return nil, err }
		socios = append(socios, socio)
	}
	return socios, nil
}

func (s *Storage) AddSocio(socio models.Socio) error {
	sql := `INSERT INTO socios (empresa_id, nome, telefone, idade, email, cpf) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.Dbpool.Exec(context.Background(), sql, socio.EmpresaID, socio.Nome, socio.Telefone, socio.Idade, socio.Email, socio.CPF)
	return err
}

func (s *Storage) DeleteSocioByID(id string) error {
	_, err := s.Dbpool.Exec(context.Background(), "DELETE FROM socios WHERE id = $1", id)
	return err
}
