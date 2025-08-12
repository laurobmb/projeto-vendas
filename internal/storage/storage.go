package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"projeto-vendas/internal/models"
	"github.com/joho/godotenv"

)

// Store é a interface que define todas as funções da nossa camada de acesso a dados.
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
	UpsertStockQuantity(productID, filialID string, quantity int) error
	AddUser(user models.User, password string) error
	AddProduct(product models.Product) error
	UpdateProduct(productID string, product models.Product) error
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
	GetSalesSummary() ([]models.SalesSummary, error)
	FilterProducts(category string, minPrice float64) ([]models.Product, error)
	GetLowStockProducts(filialNome string, limit int) ([]models.LowStockProduct, error)
	GetTopBillingBranch(period string) (*models.TopBillingBranch, error)
	GetSalesSummaryByBranch(period string, branchName string) (*models.BranchSalesSummary, error)
	GetTopSellerByPeriod(period string) (*models.TopSeller, error)
	GetDailySalesByBranch(days int) ([]models.DailyBranchSales, error)
	GetDashboardMetrics(days int) (float64, int, error)
	GetFinancialKPIs(days int) (models.FinancialKPIs, error)

	GetTopSellers(days int) ([]models.TopSeller, error)
	GetTotalStockValue() (float64, error)
	GetStockComposition() ([]models.StockComposition, error)
	GetProductDetails(identifier string) (*models.Product, error)

}

type Storage struct {
	Dbpool *pgxpool.Pool
}

func NewStorage() (*Storage, error) {
	if err := godotenv.Load("config.env"); err != nil {
		if err := godotenv.Load(); err != nil {
			log.Println("Aviso: Nenhum ficheiro .env ou config.env encontrado.")
		}
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("não foi possível conectar ao banco de dados: %w", err)
	}
	return &Storage{Dbpool: pool}, nil
}

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
	sqlProduct := `INSERT INTO produtos (nome, descricao, categoria, codigo_barras, codigo_cnae, preco_custo, percentual_lucro, imposto_estadual, imposto_federal, preco_sugerido) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	err = tx.QueryRow(context.Background(), sqlProduct, product.Nome, product.Descricao, product.Categoria, product.CodigoBarras, product.CodigoCNAE, product.PrecoCusto, product.PercentualLucro, product.ImpostoEstadual, product.ImpostoFederal, product.PrecoSugerido).Scan(&newProductID)
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
		SELECT p.id, p.nome, p.codigo_barras, COALESCE(p.codigo_cnae, ''), COALESCE(p.categoria, ''), f.id, f.nome, ef.quantidade
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
		if err := rows.Scan(&item.ProdutoID, &item.ProdutoNome, &item.CodigoBarras, &item.CodigoCNAE, &item.Categoria, &item.FilialID, &item.FilialNome, &item.Quantidade); err != nil {
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
		SELECT p.id, p.nome, p.descricao, COALESCE(p.categoria, ''), p.codigo_barras, COALESCE(p.codigo_cnae, ''), p.preco_custo, p.percentual_lucro,
		p.imposto_estadual, p.imposto_federal, p.preco_sugerido,
				COALESCE(SUM(ef.quantidade), 0) as total_estoque,
				(COALESCE(SUM(ef.quantidade), 0) * p.preco_sugerido) as valor_total_estoque
		FROM produtos p
		LEFT JOIN estoque_filiais ef ON p.id = ef.produto_id
`
	var args []interface{}
	placeholderCount := 1
	if searchQuery != "" {
		sql += fmt.Sprintf(" WHERE p.nome ILIKE $%d OR p.codigo_barras ILIKE $%d", placeholderCount, placeholderCount+1)
		args = append(args, "%"+searchQuery+"%", "%"+searchQuery+"%")
		placeholderCount += 2
	}
	sql += fmt.Sprintf(" GROUP BY p.id ORDER BY p.nome LIMIT $%d OFFSET $%d", placeholderCount, placeholderCount+1)
	args = append(args, limit, offset)
	rows, err := s.Dbpool.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Nome, &p.Descricao, &p.Categoria, &p.CodigoBarras, &p.CodigoCNAE, &p.PrecoCusto, &p.PercentualLucro, &p.ImpostoEstadual, &p.ImpostoFederal, &p.PrecoSugerido, &p.TotalEstoque, &p.ValorTotalEstoque); err != nil {
			return nil, err
		}
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
		SELECT f.id, f.nome, COALESCE(ef.quantidade, 0) as quantidade
		FROM filiais f
		LEFT JOIN estoque_filiais ef ON f.id = ef.filial_id AND ef.produto_id = $1
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

func (s *Storage) UpsertStockQuantity(productID, filialID string, quantity int) error {
	sql := `
		INSERT INTO estoque_filiais (produto_id, filial_id, quantidade)
		VALUES ($1, $2, $3)
		ON CONFLICT (produto_id, filial_id)
		DO UPDATE SET quantidade = $3
	`
	_, err := s.Dbpool.Exec(context.Background(), sql, productID, filialID, quantity)
	return err
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
	sql := `INSERT INTO produtos (nome, descricao, categoria, codigo_barras, codigo_cnae, preco_custo, percentual_lucro, imposto_estadual, imposto_federal, preco_sugerido) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := s.Dbpool.Exec(context.Background(), sql, product.Nome, product.Descricao, product.Categoria, product.CodigoBarras, product.CodigoCNAE, product.PrecoCusto, product.PercentualLucro, product.ImpostoEstadual, product.ImpostoFederal, product.PrecoSugerido)
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

func (s *Storage) UpdateProduct(productID string, product models.Product) error {
    sql := `
        UPDATE produtos SET 
            nome = $1, descricao = $2, categoria = $3, codigo_barras = $4, preco_custo = $5, 
            percentual_lucro = $6, imposto_estadual = $7, imposto_federal = $8, preco_sugerido = $9, codigo_cnae = $10,
            data_atualizacao = NOW()
        WHERE id = $11
	`
    cmdTag, err := s.Dbpool.Exec(context.Background(), sql, 
        product.Nome, product.Descricao, product.Categoria, product.CodigoBarras, product.PrecoCusto,
        product.PercentualLucro, product.ImpostoEstadual, product.ImpostoFederal, product.PrecoSugerido, product.CodigoCNAE,
        productID)
    
    if err != nil { return err }
    if cmdTag.RowsAffected() == 0 { return errors.New("nenhum produto foi atualizado (ID não encontrado?)") }
    return nil
}

func (s *Storage) FilterProducts(category string, minPrice float64) ([]models.Product, error) {
    var products []models.Product
    sql := `
        SELECT id, nome, preco_sugerido 
        FROM produtos 
        WHERE nome ILIKE $1 AND preco_sugerido > $2
        ORDER BY preco_sugerido DESC
        LIMIT 10
    `
    rows, err := s.Dbpool.Query(context.Background(), sql, "%"+category+"%", minPrice)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var p models.Product
        if err := rows.Scan(&p.ID, &p.Nome, &p.PrecoSugerido); err != nil {
            return nil, err
        }
        products = append(products, p)
    }
    return products, nil
}

func (s *Storage) GetTopSellers(days int) ([]models.TopSeller, error) {
	var sellers []models.TopSeller
	sql := `
		SELECT u.nome, SUM(v.total_venda) as total
		FROM vendas v
		JOIN usuarios u ON v.usuario_id = u.id
		WHERE v.data_venda >= CURRENT_DATE - MAKE_INTERVAL(days => $1) AND u.cargo = 'vendedor'
		GROUP BY u.nome
		ORDER BY total DESC
		LIMIT 3
	`
	rows, err := s.Dbpool.Query(context.Background(), sql, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var seller models.TopSeller
		if err := rows.Scan(&seller.VendedorNome, &seller.TotalVendas); err != nil {
			return nil, err
		}
		sellers = append(sellers, seller)
	}
	return sellers, nil
}


func (s *Storage) GetLowStockProducts(filialNome string, limit int) ([]models.LowStockProduct, error) {
	var products []models.LowStockProduct
	sql := `
		SELECT p.nome, f.nome, ef.quantidade
		FROM estoque_filiais ef
		JOIN produtos p ON ef.produto_id = p.id
		JOIN filiais f ON ef.filial_id = f.id
	`
	args := []interface{}{}
	placeholderCount := 1

	if filialNome != "" {
		sql += fmt.Sprintf(" WHERE f.nome ILIKE $%d", placeholderCount)
		args = append(args, filialNome)
		placeholderCount++
	}

	sql += fmt.Sprintf(" ORDER BY ef.quantidade ASC LIMIT $%d", placeholderCount)
	args = append(args, limit)

	rows, err := s.Dbpool.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.LowStockProduct
		if err := rows.Scan(&p.ProdutoNome, &p.FilialNome, &p.Quantidade); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (s *Storage) GetTopBillingBranch(period string) (*models.TopBillingBranch, error) {
	var result models.TopBillingBranch
	sql := `
		SELECT f.nome, SUM(v.total_venda) as total
		FROM vendas v
		JOIN filiais f ON v.filial_id = f.id
		WHERE v.data_venda >= date_trunc($1, CURRENT_DATE)
		GROUP BY f.nome
		ORDER BY total DESC
		LIMIT 1;
	`
	err := s.Dbpool.QueryRow(context.Background(), sql, period).Scan(&result.FilialNome, &result.TotalFaturado)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Retorna nil se não houver vendas no período
		}
		return nil, err
	}
	return &result, nil
}

// NOVO: Obtém um resumo de vendas para uma filial específica num período.
func (s *Storage) GetSalesSummaryByBranch(period string, branchName string) (*models.BranchSalesSummary, error) {
	var result models.BranchSalesSummary
	sql := `
		SELECT f.nome, COALESCE(SUM(v.total_venda), 0), COUNT(v.id)
		FROM filiais f
		LEFT JOIN vendas v ON f.id = v.filial_id AND v.data_venda >= date_trunc($2, CURRENT_DATE)
		WHERE f.nome ILIKE $1
		GROUP BY f.nome;
	`
	err := s.Dbpool.QueryRow(context.Background(), sql, branchName, period).Scan(&result.FilialNome, &result.TotalVendas, &result.NumeroTransacoes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("filial '%s' não encontrada", branchName)
		}
		return nil, err
	}
	if result.NumeroTransacoes > 0 {
		result.TicketMedio = result.TotalVendas / float64(result.NumeroTransacoes)
	}
	return &result, nil
}

// NOVO: Obtém o melhor vendedor num determinado período.
func (s *Storage) GetTopSellerByPeriod(period string) (*models.TopSeller, error) {
	var seller models.TopSeller
	sql := `
		SELECT u.nome, SUM(v.total_venda) as total
		FROM vendas v
		JOIN usuarios u ON v.usuario_id = u.id
		WHERE v.data_venda >= date_trunc($1, CURRENT_DATE) AND u.cargo = 'vendedor'
		GROUP BY u.nome
		ORDER BY total DESC
		LIMIT 1;
	`
	err := s.Dbpool.QueryRow(context.Background(), sql, period).Scan(&seller.VendedorNome, &seller.TotalVendas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Retorna nil se não houver vendas no período
		}
		return nil, err
	}
	return &seller, nil
}

// NOVO: Obtém as vendas diárias agrupadas por filial dos últimos N dias.
func (s *Storage) GetDailySalesByBranch(days int) ([]models.DailyBranchSales, error) {
	var sales []models.DailyBranchSales
	sql := `
		SELECT 
			TO_CHAR(v.data_venda, 'YYYY-MM-DD') as dia,
			f.nome as filial_nome,
			SUM(v.total_venda) as total_vendas
		FROM vendas v
		JOIN filiais f ON v.filial_id = f.id
		WHERE v.data_venda >= CURRENT_DATE - MAKE_INTERVAL(days => $1)
		GROUP BY dia, f.nome
		ORDER BY dia, f.nome;
	`
	rows, err := s.Dbpool.Query(context.Background(), sql, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sale models.DailyBranchSales
		if err := rows.Scan(&sale.Date, &sale.FilialNome, &sale.TotalVendas); err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	return sales, nil
}

// NOVO: Obtém as métricas gerais do dashboard (faturamento total, transações).
func (s *Storage) GetDashboardMetrics(days int) (float64, int, error) {
	var totalRevenue float64
	var totalTransactions int
	sql := `
		SELECT COALESCE(SUM(total_venda), 0), COUNT(id)
		FROM vendas
		WHERE data_venda >= CURRENT_DATE - MAKE_INTERVAL(days => $1);
	`
	err := s.Dbpool.QueryRow(context.Background(), sql, days).Scan(&totalRevenue, &totalTransactions)
	return totalRevenue, totalTransactions, err
}


// NOVO: Calcula o valor total de todos os produtos em stock.
func (s *Storage) GetTotalStockValue() (float64, error) {
	var totalValue float64
	sql := `
		SELECT COALESCE(SUM(p.preco_custo * ef.quantidade), 0) 
		FROM estoque_filiais ef
		JOIN produtos p ON ef.produto_id = p.id;
	`
	err := s.Dbpool.QueryRow(context.Background(), sql).Scan(&totalValue)
	return totalValue, err
}

func (s *Storage) GetStockComposition() ([]models.StockComposition, error) {
	var composition []models.StockComposition
	sql := `
		SELECT 
			COALESCE(p.categoria, 'Sem Categoria') as categoria,
			SUM(p.preco_custo * ef.quantidade) as valor
		FROM produtos p
		JOIN estoque_filiais ef ON p.id = ef.produto_id
		GROUP BY categoria
		ORDER BY valor DESC;
	`
	rows, err := s.Dbpool.Query(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.StockComposition
		if err := rows.Scan(&item.Category, &item.Value); err != nil {
			return nil, err
		}
		composition = append(composition, item)
	}
	return composition, nil
}

// NOVO: Calcula a Margem de Lucro Bruta e o Giro de Estoque.
func (s *Storage) GetFinancialKPIs(days int) (models.FinancialKPIs, error) {
	var kpis models.FinancialKPIs
	var totalRevenue, costOfGoodsSold, avgInventoryValue float64

	// 1. Calcula o Faturamento Total e o Custo dos Produtos Vendidos (COGS)
	sqlCogs := `
		SELECT 
			COALESCE(SUM(iv.preco_unitario * iv.quantidade), 0) as revenue,
			COALESCE(SUM(p.preco_custo * iv.quantidade), 0) as cogs
		FROM itens_venda iv
		JOIN vendas v ON iv.venda_id = v.id
		JOIN produtos p ON iv.produto_id = p.id
		WHERE v.data_venda >= CURRENT_DATE - MAKE_INTERVAL(days => $1);
	`
	err := s.Dbpool.QueryRow(context.Background(), sqlCogs, days).Scan(&totalRevenue, &costOfGoodsSold)
	if err != nil {
		return kpis, fmt.Errorf("falha ao calcular COGS e faturamento: %w", err)
	}

	// 2. Calcula o Valor Médio do Estoque (simplificado como o valor atual)
	sqlAvgInventory := `
		SELECT COALESCE(SUM(p.preco_custo * ef.quantidade), 0)
		FROM estoque_filiais ef
		JOIN produtos p ON ef.produto_id = p.id;
	`
	err = s.Dbpool.QueryRow(context.Background(), sqlAvgInventory).Scan(&avgInventoryValue)
	if err != nil {
		return kpis, fmt.Errorf("falha ao calcular valor do estoque: %w", err)
	}

	// 3. Calcula os KPIs
	if totalRevenue > 0 {
		kpis.GrossProfitMargin = (totalRevenue - costOfGoodsSold) / totalRevenue * 100
	}
	if avgInventoryValue > 0 {
		kpis.InventoryTurnover = costOfGoodsSold / avgInventoryValue
	}

	return kpis, nil
}

func (s *Storage) GetProductDetails(identifier string) (*models.Product, error) {
	var p models.Product
	sql := `
		SELECT 
			p.id, p.nome, p.descricao, COALESCE(p.categoria, ''), p.codigo_barras, COALESCE(p.codigo_cnae, ''), 
			p.preco_custo, p.percentual_lucro, p.imposto_estadual, p.imposto_federal, p.preco_sugerido,
			COALESCE(SUM(ef.quantidade), 0) as total_estoque
		FROM produtos p
		LEFT JOIN estoque_filiais ef ON p.id = ef.produto_id
		WHERE p.codigo_barras = $1 OR p.codigo_cnae = $1
		GROUP BY p.id
		LIMIT 1;
	`
	err := s.Dbpool.QueryRow(context.Background(), sql, identifier).Scan(
		&p.ID, &p.Nome, &p.Descricao, &p.Categoria, &p.CodigoBarras, &p.CodigoCNAE,
		&p.PrecoCusto, &p.PercentualLucro, &p.ImpostoEstadual, &p.ImpostoFederal, &p.PrecoSugerido,
		&p.TotalEstoque, // Adicionado o scan para o estoque
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

