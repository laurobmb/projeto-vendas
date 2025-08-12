package models

import (
	"github.com/google/uuid"
	"time"
)

// User representa a estrutura de um utilizador no sistema.
type User struct {
	ID        uuid.UUID
	Nome      string
	Email     string
	Cargo     string
	SenhaHash string
	FilialID  *uuid.UUID
}

// Product representa a estrutura de um produto no catálogo.
type Product struct {
	ID                uuid.UUID
	Nome              string
	Descricao         string
	Categoria       string // NOVO
	CodigoBarras      string
	CodigoCNAE        string `json:"CodigoCNAE,omitempty"`
	PrecoCusto        float64 // NOVO
	PercentualLucro   float64 // NOVO
	ImpostoEstadual   float64 // NOVO
	ImpostoFederal    float64 // NOVO
	PrecoSugerido     float64
	TotalEstoque      int
	ValorTotalEstoque float64
}

// Filial representa uma loja ou supermercado.
type Filial struct {
	ID   uuid.UUID
	Nome string
}

// StockDetail representa o detalhe do stock de um produto numa filial.
type StockDetail struct {
	FilialID   uuid.UUID
	FilialNome string
	Quantidade int
}

// PaginationData armazena informações para renderizar controlos de paginação.
type PaginationData struct {
	HasPrev, HasNext         bool
	PrevPage, NextPage       int
	CurrentPage, TotalPages  int
	SearchQuery              string
	FilterID                 string
}

// StockViewItem representa uma linha na nova tabela de gestão de stock.
type StockViewItem struct {
	ProdutoID    uuid.UUID
	ProdutoNome  string
	CodigoBarras string
	CodigoCNAE   string // NOVO
	Categoria    string // NOVO
	FilialID     uuid.UUID
	FilialNome   string
	Quantidade   int
}

// Venda representa o registo de uma transação.
type Venda struct {
	ID         uuid.UUID
	UsuarioID  uuid.UUID
	FilialID   uuid.UUID
	TotalVenda float64
	DataVenda  time.Time
}

// ItemVenda representa um item dentro de uma venda.
type ItemVenda struct {
	ProdutoIDStr  string    `json:"product_id"`
	ProdutoID     uuid.UUID `json:"-"`
	Quantidade    int       `json:"quantity"`
	PrecoUnitario float64   `json:"unit_price"`
}

// SaleReportItem representa uma linha no novo relatório de vendas.
type SaleReportItem struct {
	VendaID      uuid.UUID
	DataVenda    time.Time
	FilialNome   string
	VendedorNome string
	TotalVenda   float64
}

// Empresa representa os dados da empresa.
type Empresa struct {
	ID           uuid.UUID
	RazaoSocial  string
	NomeFantasia string
	CNPJ         string
	Endereco     string
}

// Socio representa os dados de um sócio.
type Socio struct {
	ID        uuid.UUID
	EmpresaID uuid.UUID
	Nome      string
	Telefone  string
	Idade     int
	Email     string
	CPF       string
}

// CORREÇÃO: Adicionada a struct que estava em falta.
// SalesSummary representa um item no resumo de vendas para a IA.
type SalesSummary struct {
	FilialNome  string  `json:"filial_nome"`
	TotalVendas float64 `json:"total_vendas"`
}

type TopSeller struct {
	VendedorNome string  `json:"vendedor_nome"`
	TotalVendas  float64 `json:"total_vendas"`
}

type LowStockProduct struct {
	ProdutoNome  string `json:"produto_nome"`
	FilialNome   string `json:"filial_nome"`
	Quantidade   int    `json:"quantidade"`
}

type BranchSalesSummary struct {
	FilialNome           string  `json:"filial_nome"`
	TotalVendas          float64 `json:"total_vendas"`
	NumeroTransacoes     int     `json:"numero_transacoes"`
	TicketMedio          float64 `json:"ticket_medio"`
}

// NOVO: Struct para a filial com maior faturamento.
type TopBillingBranch struct {
	FilialNome    string  `json:"filial_nome"`
	TotalFaturado float64 `json:"total_faturado"`
}

// NOVO: Struct para os dados de vendas diárias por filial para o gráfico.
type DailyBranchSales struct {
	Date         string  `json:"date"`
	FilialNome   string  `json:"filial_nome"`
	TotalVendas  float64 `json:"total_vendas"`
}

type StockComposition struct {
	Category string  `json:"category"`
	Value    float64 `json:"value"`
}

type FinancialKPIs struct {
	GrossProfitMargin float64 `json:"gross_profit_margin"`
	InventoryTurnover float64 `json:"inventory_turnover"`
}

// ATUALIZADO: A struct principal do dashboard agora inclui os novos KPIs financeiros.
type MonitoringDashboardData struct {
	SalesByBranch     []DailyBranchSales
	TopSellers        []TopSeller
	StockComposition  []StockComposition
	LowStockAlerts    []LowStockProduct
	TotalRevenue      float64
	TotalTransactions int
	AverageTicket     float64
	TotalStockValue   float64
	FinancialKPIs     FinancialKPIs // NOVO
}
