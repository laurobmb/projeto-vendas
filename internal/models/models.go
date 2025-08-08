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
	CodigoBarras      string
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
	ProdutoID   uuid.UUID
	ProdutoNome string
	CodigoBarras string // CORREÇÃO: Adicionado o campo que estava em falta.
	FilialID    uuid.UUID
	FilialNome  string
	Quantidade  int
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

// NOVO: SaleReportItem representa uma linha no novo relatório de vendas.
type SaleReportItem struct {
	VendaID      uuid.UUID
	DataVenda    time.Time
	FilialNome   string
	VendedorNome string
	TotalVenda   float64
}
