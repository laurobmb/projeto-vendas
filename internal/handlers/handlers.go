package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"projeto-vendas/internal/models"
	"projeto-vendas/internal/storage"
)

const PageLimit = 10
type Handler struct {
	Storage storage.Store
}
func NewHandler(s storage.Store) *Handler {
	return &Handler{Storage: s}
}

// getFlashes lê e apaga as mensagens flash da sessão.
func getFlashes(c *gin.Context) gin.H {
	session := sessions.Default(c)
	successFlashes := session.Flashes("success")
	errorFlashes := session.Flashes("error")
	session.Save() // Salva a sessão para limpar as flashes

	data := gin.H{}
	if len(successFlashes) > 0 {
		data["success"] = successFlashes[0]
	}
	if len(errorFlashes) > 0 {
		data["error"] = errorFlashes[0]
	}
	return data
}

func (h *Handler) AuthRequired(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userRole, ok := session.Get("userRole").(string)
		isAPIRequest := strings.HasPrefix(c.Request.URL.Path, "/api/")

		if !ok {
			if isAPIRequest {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			} else {
				c.Redirect(http.StatusFound, "/login")
			}
			c.Abort()
			return
		}
		
		hasPermission := false
		for _, role := range requiredRoles {
			if userRole == role {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			log.Printf("Acesso negado para o utilizador com cargo '%s'. Rota requer um dos seguintes cargos: %v", userRole, requiredRoles)
			if isAPIRequest {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Acesso negado"})
			} else {
				c.HTML(http.StatusForbidden, "error.html", gin.H{
					"title":        "Acesso Negado",
					"StatusCode":   http.StatusForbidden,
					"ErrorMessage": "Acesso Negado",
				})
			}
			c.Abort()
			return
		}
		
		c.Next()
	}
}

func (h *Handler) ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login"})
}

func (h *Handler) HandleLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	user, err := h.Storage.GetUserByEmail(email)
	if err != nil {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{"error": "Email ou senha inválidos."})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.SenhaHash), []byte(password)); err != nil {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{"error": "Email ou senha inválidos."})
		return
	}
	
	session := sessions.Default(c)
	session.Set("userID", user.ID.String())
	session.Set("userName", user.Nome)
	session.Set("userRole", user.Cargo)

	if user.FilialID != nil {
		filial, err := h.Storage.GetFilialByID(user.FilialID.String())
		if err == nil {
			session.Set("filialID", filial.ID.String())
			session.Set("filialName", filial.Nome)
		} else {
			log.Printf("Aviso: Utilizador %s tem filial_id %s mas a filial não foi encontrada.", user.Email, user.FilialID)
		}
	}

	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func (h *Handler) HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

func (h *Handler) ShowAdminDashboard(c *gin.Context) {
	session := sessions.Default(c)
	pageUsers, _ := strconv.Atoi(c.DefaultQuery("page_users", "1"))
	pageProducts, _ := strconv.Atoi(c.DefaultQuery("page_products", "1"))
	searchQuery := c.Query("search_products")
	if pageUsers < 1 { pageUsers = 1 }
	if pageProducts < 1 { pageProducts = 1 }

	totalUsers, _ := h.Storage.CountUsers()
	users, _ := h.Storage.GetUsersPaginated(PageLimit, (pageUsers-1)*PageLimit)
	totalPagesUsers := int(math.Ceil(float64(totalUsers) / float64(PageLimit)))

	totalProducts, _ := h.Storage.CountProducts(searchQuery)
	products, _ := h.Storage.GetProductsPaginatedAndFiltered(searchQuery, PageLimit, (pageProducts-1)*PageLimit)
	totalPagesProducts := int(math.Ceil(float64(totalProducts) / float64(PageLimit)))

	filiais, _ := h.Storage.GetAllFiliais()

	data := getFlashes(c)
	data["title"] = "Painel do Administrador"
	data["users"] = users
	data["products"] = products
	data["filiais"] = filiais
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["ActivePage"] = "dashboard"
	data["PaginationUsers"] = models.PaginationData{
		HasPrev:     pageUsers > 1,
		HasNext:     pageUsers < totalPagesUsers,
		PrevPage:    pageUsers - 1,
		NextPage:    pageUsers + 1,
		CurrentPage: pageUsers,
		TotalPages:  totalPagesUsers,
	}
	data["PaginationProducts"] = models.PaginationData{
		HasPrev:     pageProducts > 1,
		HasNext:     pageProducts < totalPagesProducts,
		PrevPage:    pageProducts - 1,
		NextPage:    pageProducts + 1,
		CurrentPage: pageProducts,
		TotalPages:  totalPagesProducts,
		SearchQuery: searchQuery,
	}
	c.HTML(http.StatusOK, "admin_dashboard.html", data)
}

func (h *Handler) ShowStockManagementPage(c *gin.Context) {
	session := sessions.Default(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	searchQuery := c.Query("search_product")
	filialID := c.Query("filial_id")
	if page < 1 { page = 1 }

	totalItems, _ := h.Storage.CountStockItems(filialID, searchQuery)
	stockItems, _ := h.Storage.GetStockItemsPaginated(filialID, searchQuery, PageLimit, (page-1)*PageLimit)
	totalPages := int(math.Ceil(float64(totalItems) / float64(PageLimit)))
	
	filiais, _ := h.Storage.GetAllFiliais()
	allProducts, _ := h.Storage.GetAllProductsSimple()

	data := getFlashes(c)
	data["title"] = "Gestão de Stock por Filial"
	data["stockItems"] = stockItems
	data["filiais"] = filiais
	data["allProducts"] = allProducts
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["ActivePage"] = "stock"
	data["Pagination"] = models.PaginationData{
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		CurrentPage: page,
		TotalPages:  totalPages,
		SearchQuery: searchQuery,
		FilterID:    filialID,
	}
	c.HTML(http.StatusOK, "stock_management.html", data)
}

func (h *Handler) ShowSalesTerminalPage(c *gin.Context) {
	session := sessions.Default(c)
	userRole := session.Get("userRole")
	
	data := gin.H{
		"title":      "Terminal de Vendas",
		"UserRole":   userRole,
		"UserName":   session.Get("userName"),
		"FilialName": session.Get("filialName"),
		"FilialID":   session.Get("filialID"),
		"ActivePage": "vendas",
	}

	if userRole == "admin" {
		filiais, err := h.Storage.GetAllFiliais()
		if err != nil {
			log.Printf("Erro ao buscar filiais para o terminal de vendas do admin: %v", err)
		}
		data["filiais"] = filiais
	}

	c.HTML(http.StatusOK, "terminal_vendas.html", data)
}

func (h *Handler) ShowSalesReportPage(c *gin.Context) {
	session := sessions.Default(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	filialID := c.Query("filial_id")
	if page < 1 { page = 1 }

	totalItems, _ := h.Storage.CountSales(filialID)
	sales, _ := h.Storage.GetSalesPaginated(filialID, PageLimit, (page-1)*PageLimit)
	totalPages := int(math.Ceil(float64(totalItems) / float64(PageLimit)))
	
	filiais, _ := h.Storage.GetAllFiliais()

	data := getFlashes(c)
	data["title"] = "Relatório de Vendas"
	data["sales"] = sales
	data["filiais"] = filiais
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["ActivePage"] = "sales"
	data["Pagination"] = models.PaginationData{
		HasPrev: page > 1, HasNext: page < totalPages,
		PrevPage: page - 1, NextPage: page + 1,
		CurrentPage: page, TotalPages: totalPages,
		FilterID:    filialID,
	}
	c.HTML(http.StatusOK, "sales_report.html", data)
}

func (h *Handler) ShowEstoquistaDashboard(c *gin.Context) {
	session := sessions.Default(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	searchQuery := c.Query("search_product")
	
	filialID, ok := session.Get("filialID").(string)
	if !ok {
		c.HTML(http.StatusForbidden, "error.html", gin.H{"title": "Erro", "ErrorMessage": "Utilizador não está associado a nenhuma filial."})
		return
	}

	if page < 1 { page = 1 }

	totalItems, _ := h.Storage.CountStockItems(filialID, searchQuery)
	stockItems, _ := h.Storage.GetStockItemsPaginated(filialID, searchQuery, PageLimit, (page-1)*PageLimit)
	totalPages := int(math.Ceil(float64(totalItems) / float64(PageLimit)))
	
	allProducts, _ := h.Storage.GetAllProductsSimple()
	
	data := getFlashes(c)
	data["title"] = "Painel de Stock"
	data["stockItems"] = stockItems
	data["allProducts"] = allProducts
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["FilialName"] = session.Get("filialName")
	data["FilialID"] = filialID
	data["ActivePage"] = "estoque"
	data["Pagination"] = models.PaginationData{
		HasPrev: page > 1, HasNext: page < totalPages,
		PrevPage: page - 1, NextPage: page + 1,
		CurrentPage: page, TotalPages: totalPages,
		SearchQuery: searchQuery,
	}

	c.HTML(http.StatusOK, "estoquista_dashboard.html", data)
}

func (h *Handler) HandleAddUser(c *gin.Context) {
	session := sessions.Default(c)
	user := models.User{
		Nome:  c.PostForm("name"),
		Email: c.PostForm("email"),
		Cargo: c.PostForm("role"),
	}
	password := c.PostForm("password")
	filialIDStr := c.PostForm("filial_id")

	if filialIDStr != "" {
		parsedUUID, err := uuid.Parse(filialIDStr)
		if err == nil {
			user.FilialID = &parsedUUID
		}
	}

	err := h.Storage.AddUser(user, password)
	if err != nil {
		log.Printf("Erro ao adicionar utilizador: %v", err)
		session.AddFlash(fmt.Sprintf("Falha ao adicionar utilizador: %v", err), "error")
	} else {
		session.AddFlash("Utilizador adicionado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

func calculateSuggestedPrice(custo, lucro, impostoEst, impostoFed float64) float64 {
    lucroValor := custo * (lucro / 100)
    impostoEstValor := custo * (impostoEst / 100)
    impostoFedValor := custo * (impostoFed / 100)
    return custo + lucroValor + impostoEstValor + impostoFedValor
}

func (h *Handler) HandleAddProduct(c *gin.Context) {
    session := sessions.Default(c)
    custo, _ := strconv.ParseFloat(c.PostForm("preco_custo"), 64)
    lucro, _ := strconv.ParseFloat(c.PostForm("percentual_lucro"), 64)
    impostoEst, _ := strconv.ParseFloat(c.PostForm("imposto_estadual"), 64)
    impostoFed, _ := strconv.ParseFloat(c.PostForm("imposto_federal"), 64)

    product := models.Product{
        Nome:          c.PostForm("name"),
        Descricao:     c.PostForm("description"),
		Categoria:     c.PostForm("categoria"),
        CodigoBarras:  c.PostForm("barcode"),
        CodigoCNAE:    c.PostForm("codigo_cnae"),
        PrecoCusto:    custo,
        PercentualLucro: lucro,
        ImpostoEstadual: impostoEst,
        ImpostoFederal: impostoFed,
        PrecoSugerido: calculateSuggestedPrice(custo, lucro, impostoEst, impostoFed),
    }

    filialID := c.PostForm("filial_id")
    quantityStr := c.PostForm("quantity")
    var err error

    if filialID != "" && quantityStr != "" {
        quantity, _ := strconv.Atoi(quantityStr)
        if quantity > 0 {
            err = h.Storage.CreateProductWithInitialStock(product, filialID, quantity)
        } else {
            err = h.Storage.AddProduct(product)
        }
    } else {
        err = h.Storage.AddProduct(product)
    }
    
    if err != nil {
        log.Printf("Erro ao adicionar produto: %v", err)
        session.AddFlash(fmt.Sprintf("Falha ao adicionar produto: %v", err), "error")
    } else {
        session.AddFlash("Produto adicionado com sucesso!", "success")
    }
    session.Save()
    c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (h *Handler) HandleEditProduct(c *gin.Context) {
    session := sessions.Default(c)
    productID := c.Param("id")
    
    custo, _ := strconv.ParseFloat(c.PostForm("preco_custo"), 64)
    lucro, _ := strconv.ParseFloat(c.PostForm("percentual_lucro"), 64)
    impostoEst, _ := strconv.ParseFloat(c.PostForm("imposto_estadual"), 64)
    impostoFed, _ := strconv.ParseFloat(c.PostForm("imposto_federal"), 64)

    product := models.Product{
        Nome:          c.PostForm("name"),
        Descricao:     c.PostForm("description"),
		Categoria:     c.PostForm("categoria"),
        CodigoBarras:  c.PostForm("barcode"),
        CodigoCNAE:    c.PostForm("codigo_cnae"),
        PrecoCusto:    custo,
        PercentualLucro: lucro,
        ImpostoEstadual: impostoEst,
        ImpostoFederal: impostoFed,
        PrecoSugerido: calculateSuggestedPrice(custo, lucro, impostoEst, impostoFed),
    }
    
    err := h.Storage.UpdateProduct(productID, product)
    if err != nil {
        log.Printf("Erro ao atualizar produto: %v", err)
        session.AddFlash(fmt.Sprintf("Falha ao atualizar produto: %v", err), "error")
    } else {
        session.AddFlash("Produto atualizado com sucesso!", "success")
    }
    session.Save()
    c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (h *Handler) HandleDeleteUser(c *gin.Context) {
	session := sessions.Default(c)
	err := h.Storage.DeleteUserByID(c.Param("id"))
	if err != nil {
		log.Printf("Erro ao apagar utilizador: %v", err)
		session.AddFlash(fmt.Sprintf("Falha ao apagar utilizador: %v", err), "error")
	} else {
		session.AddFlash("Utilizador removido com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
}

func (h *Handler) HandleDeleteProduct(c *gin.Context) {
	err := h.Storage.DeleteProductByID(c.Param("id"))
	if err != nil { log.Printf("Erro ao apagar produto: %v", err) }
	c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
}

func (h *Handler) HandleUpdateStock(c *gin.Context) {
	productID := c.PostForm("product_id")
	filialID := c.PostForm("filial_id")
	newQuantity, err := strconv.Atoi(c.PostForm("quantity"))
	if err != nil || newQuantity < 0 {
		log.Println("Erro: Quantidade inválida.")
		c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
		return
	}
	err = h.Storage.UpdateStockQuantity(productID, filialID, newQuantity)
	if err != nil {
		log.Printf("Erro ao atualizar stock: %v", err)
	}
	c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
}

func (h *Handler) HandleAddStockItem(c *gin.Context) {
	session := sessions.Default(c)
	addType := c.PostForm("add_type")
	filialID := c.PostForm("filial_id")
	quantity, err := strconv.Atoi(c.PostForm("quantity"))

	if err != nil || quantity < 0 {
		session.AddFlash("Quantidade inválida.", "error")
		session.Save()
		c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
		return
	}

	if addType == "new" {
		custo, _ := strconv.ParseFloat(c.PostForm("new_product_price"), 64)
        lucro, _ := strconv.ParseFloat(c.PostForm("new_product_lucro"), 64)
        impostoEst, _ := strconv.ParseFloat(c.PostForm("new_product_imposto_est"), 64)
        impostoFed, _ := strconv.ParseFloat(c.PostForm("new_product_imposto_fed"), 64)

		newProduct := models.Product{
			Nome:          c.PostForm("new_product_name"),
			Descricao:     c.PostForm("new_product_description"),
			CodigoBarras:  c.PostForm("new_product_barcode"),
			PrecoCusto:    custo,
            PercentualLucro: lucro,
            ImpostoEstadual: impostoEst,
            ImpostoFederal: impostoFed,
            PrecoSugerido: calculateSuggestedPrice(custo, lucro, impostoEst, impostoFed),
		}
		err = h.Storage.CreateProductWithInitialStock(newProduct, filialID, quantity)
	} else {
		productID := c.PostForm("product_id")
		err = h.Storage.AddStockItem(productID, filialID, quantity)
	}

	if err != nil {
		log.Printf("Erro ao processar adição de stock: %v", err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			session.AddFlash(fmt.Sprintf("Erro: O produto com este nome ou código de barras já existe. Use a opção 'Adicionar a Produto Existente'."), "error")
		} else {
			session.AddFlash(fmt.Sprintf("Falha ao adicionar stock: %v", err), "error")
		}
	} else {
		session.AddFlash("Stock adicionado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
}

func (h *Handler) HandleGetProductStock(c *gin.Context) {
	productID := c.Param("id")
	stockDetails, err := h.Storage.GetProductStockByFilial(productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter stock do produto"})
		return
	}
	c.JSON(http.StatusOK, stockDetails)
}

func (h *Handler) HandleSetStock(c *gin.Context) {
	session := sessions.Default(c)
	productID := c.PostForm("product_id")
	filialID := c.PostForm("filial_id")
	quantity, err := strconv.Atoi(c.PostForm("quantity"))

	if err != nil || quantity < 0 {
		session.AddFlash("Quantidade inválida.", "error")
		session.Save()
		c.Redirect(http.StatusFound, c.Request.Header.Get("Referer"))
		return
	}

	err = h.Storage.UpsertStockQuantity(productID, filialID, quantity)
	if err != nil {
		log.Printf("Erro ao definir stock: %v", err)
		session.AddFlash(fmt.Sprintf("Falha ao definir stock: %v", err), "error")
	} else {
		session.AddFlash("Stock atualizado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (h *Handler) HandleSearchProductsForSale(c *gin.Context) {
	query := c.Query("q")
	filialIDStr := c.Query("filial_id")
	if filialIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da filial é obrigatório."})
		return
	}
	filialID, err := uuid.Parse(filialIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da filial inválido."})
		return
	}

	products, err := h.Storage.SearchProductsForSale(query, filialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar produtos."})
		return
	}
	c.JSON(http.StatusOK, products)
}

func (h *Handler) HandleRegisterSale(c *gin.Context) {
	session := sessions.Default(c)
	userID, _ := uuid.Parse(session.Get("userID").(string))
	
	var req struct {
		FilialID string               `json:"filial_id"`
		Items    []models.ItemVenda `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados da venda inválidos."})
		return
	}
	
	filialID, err := uuid.Parse(req.FilialID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da filial na venda é inválido."})
		return
	}

	var total float64
	for i := range req.Items {
		req.Items[i].ProdutoID, _ = uuid.Parse(req.Items[i].ProdutoIDStr)
		total += req.Items[i].PrecoUnitario * float64(req.Items[i].Quantidade)
	}

	venda := models.Venda{
		UsuarioID:  userID,
		FilialID:   filialID,
		TotalVenda: total,
	}

	if err := h.Storage.RegisterSale(venda, req.Items); err != nil {
		log.Printf("Erro ao registar venda: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) HandleEditUser(c *gin.Context) {
    session := sessions.Default(c)
    userID := c.Param("id")

    user := models.User{
        Nome:  c.PostForm("name"),
        Email: c.PostForm("email"),
        Cargo: c.PostForm("role"),
    }
    newPassword := c.PostForm("password")
    filialIDStr := c.PostForm("filial_id")

    if filialIDStr != "" {
        parsedUUID, err := uuid.Parse(filialIDStr)
        if err == nil {
            user.FilialID = &parsedUUID
        }
    }

    err := h.Storage.UpdateUser(userID, user, newPassword)
    if err != nil {
        log.Printf("Erro ao atualizar utilizador: %v", err)
        session.AddFlash(fmt.Sprintf("Falha ao atualizar utilizador: %v", err), "error")
    } else {
        session.AddFlash("Utilizador atualizado com sucesso!", "success")
    }
    session.Save()
    c.Redirect(http.StatusFound, "/admin/dashboard")
}

func (h *Handler) ShowEmpresaPage(c *gin.Context) {
	session := sessions.Default(c)
	empresa, err := h.Storage.GetEmpresa()
	if err != nil {
		log.Printf("Erro ao obter dados da empresa: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"title": "Erro", "ErrorMessage": "Não foi possível carregar os dados da empresa."})
		return
	}
	
	socios, err := h.Storage.GetSocios(empresa.ID)
	if err != nil {
		log.Printf("Erro ao obter sócios: %v", err)
	}

	data := getFlashes(c)
	data["title"] = "Dados da Empresa"
	data["empresa"] = empresa
	data["socios"] = socios
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["ActivePage"] = "empresa"
	c.HTML(http.StatusOK, "empresa.html", data)
}

func (h *Handler) HandleUpdateEmpresa(c *gin.Context) {
	session := sessions.Default(c)
	empresaIDStr := c.PostForm("id")
	var empresaID uuid.UUID
	var err error
	if empresaIDStr != "" {
		empresaID, err = uuid.Parse(empresaIDStr)
		if err != nil {
			log.Printf("ID da empresa inválido: %v", err)
		}
	}
	
	empresa := models.Empresa{
		ID:           empresaID,
		RazaoSocial:  c.PostForm("razao_social"),
		NomeFantasia: c.PostForm("nome_fantasia"),
		CNPJ:         c.PostForm("cnpj"),
		Endereco:     c.PostForm("endereco"),
	}

	err = h.Storage.UpsertEmpresa(empresa)
	if err != nil {
		session.AddFlash(fmt.Sprintf("Falha ao atualizar dados da empresa: %v", err), "error")
	} else {
		session.AddFlash("Dados da empresa atualizados com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/empresa")
}

func (h *Handler) HandleAddSocio(c *gin.Context) {
	session := sessions.Default(c)
	empresaID, _ := uuid.Parse(c.PostForm("empresa_id"))
	idade, _ := strconv.Atoi(c.PostForm("idade"))

	socio := models.Socio{
		EmpresaID: empresaID,
		Nome:      c.PostForm("nome"),
		Telefone:  c.PostForm("telefone"),
		Idade:     idade,
		Email:     c.PostForm("email"),
		CPF:       c.PostForm("cpf"),
	}

	err := h.Storage.AddSocio(socio)
	if err != nil {
		session.AddFlash(fmt.Sprintf("Falha ao adicionar sócio: %v", err), "error")
	} else {
		session.AddFlash("Sócio adicionado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/empresa")
}

func (h *Handler) HandleDeleteSocio(c *gin.Context) {
	session := sessions.Default(c)
	err := h.Storage.DeleteSocioByID(c.Param("id"))
	if err != nil {
		session.AddFlash(fmt.Sprintf("Falha ao apagar sócio: %v", err), "error")
	} else {
		session.AddFlash("Sócio apagado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/empresa")
}

func (h *Handler) HandleEditSocio(c *gin.Context) {
	session := sessions.Default(c)
	socioID := c.Param("id")
	idade, _ := strconv.Atoi(c.PostForm("idade"))

	socio := models.Socio{
		Nome:     c.PostForm("nome"),
		Telefone: c.PostForm("telefone"),
		Idade:    idade,
		Email:    c.PostForm("email"),
		CPF:      c.PostForm("cpf"),
	}

	err := h.Storage.UpdateSocio(socioID, socio)
	if err != nil {
		session.AddFlash(fmt.Sprintf("Falha ao atualizar sócio: %v", err), "error")
	} else {
		session.AddFlash("Sócio atualizado com sucesso!", "success")
	}
	session.Save()
	c.Redirect(http.StatusFound, "/admin/empresa")
}

func (h *Handler) HandleGetSalesSummary(c *gin.Context) {
    summary, err := h.Storage.GetSalesSummary()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter o resumo de vendas."})
        return
    }
    c.JSON(http.StatusOK, summary)
}

func (h *Handler) HandleFilterProducts(c *gin.Context) {
	category := c.Query("category")
	minPriceStr := c.Query("min_price")
	
	minPrice, err := strconv.ParseFloat(minPriceStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetro 'min_price' inválido."})
		return
	}

	products, err := h.Storage.FilterProducts(category, minPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao filtrar produtos."})
		return
	}
	c.JSON(http.StatusOK, products)
}

func (h *Handler) HandleAIChat(c *gin.Context) {
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Corpo do pedido inválido"})
		return
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chave de API da Gemini não está configurada no servidor."})
		return
	}

	// ATUALIZADO: O modelo agora é lido a partir das variáveis de ambiente.
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash-preview-05-20" // Modelo padrão
	}
	
	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

	payloadBytes, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao criar o payload."})
		return
	}

	resp, err := http.Post(geminiURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao comunicar com a API da Gemini."})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao ler a resposta da API da Gemini."})
		return
	}

	c.Data(resp.StatusCode, "application/json", body)
}

func (h *Handler) HandleGetTopSellers(c *gin.Context) {
	// CORREÇÃO: Lê o parâmetro 'period' do URL e passa-o para a função do storage.
	periodStr := c.DefaultQuery("period", "30") // Padrão de 30 dias se não for especificado
	period, err := strconv.Atoi(periodStr)
	if err != nil || period <= 0 {
		period = 30 // Fallback para o padrão em caso de erro
	}

	sellers, err := h.Storage.GetTopSellers(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter o ranking de vendedores."})
		return
	}
	c.JSON(http.StatusOK, sellers)
}

func (h *Handler) HandleGetLowStockProducts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "5") // Padrão de 5 itens se não for especificado
	filialNome := c.Query("filial")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetro 'limit' inválido."})
		return
	}

	products, err := h.Storage.GetLowStockProducts(filialNome, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter produtos com stock baixo."})
		return
	}
	c.JSON(http.StatusOK, products)
}


// NOVO: Handler para a filial com maior faturamento.
func (h *Handler) HandleGetTopBillingBranch(c *gin.Context) {
	period := c.DefaultQuery("period", "month") // 'month', 'week', 'day'
	result, err := h.Storage.GetTopBillingBranch(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter dados."})
		return
	}
	c.JSON(http.StatusOK, result)
}

// NOVO: Handler para o resumo de vendas por filial.
func (h *Handler) HandleGetSalesSummaryByBranch(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	branchName := c.Query("branch")
	if branchName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O nome da filial é obrigatório."})
		return
	}
	result, err := h.Storage.GetSalesSummaryByBranch(period, branchName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// NOVO: Handler para o melhor vendedor.
func (h *Handler) HandleGetTopSellerByPeriod(c *gin.Context) {
	period := c.DefaultQuery("period", "day")
	result, err := h.Storage.GetTopSellerByPeriod(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao obter dados."})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ATUALIZADO: O handler agora busca todos os dados para o dashboard completo.
func (h *Handler) ShowMonitoringDashboard(c *gin.Context) {
	session := sessions.Default(c)
	
	periodStr := c.DefaultQuery("period", "30")
	period, _ := strconv.Atoi(periodStr)
	if period == 0 {
		period = 30
	}

	salesData, _ := h.Storage.GetDailySalesByBranch(period)
	lowStock, _ := h.Storage.GetLowStockProducts("", 10)
	totalRevenue, totalTransactions, _ := h.Storage.GetDashboardMetrics(period)
	topSellers, _ := h.Storage.GetTopSellers(period)
	totalStockValue, _ := h.Storage.GetTotalStockValue()
	stockComposition, _ := h.Storage.GetStockComposition()
	financialKPIs, _ := h.Storage.GetFinancialKPIs(period) // NOVO

	var averageTicket float64
	if totalTransactions > 0 {
		averageTicket = totalRevenue / float64(totalTransactions)
	}

	dashboardData := models.MonitoringDashboardData{
		SalesByBranch:     salesData,
		TopSellers:        topSellers,
		StockComposition:  stockComposition,
		LowStockAlerts:    lowStock,
		TotalRevenue:      totalRevenue,
		TotalTransactions: totalTransactions,
		AverageTicket:     averageTicket,
		TotalStockValue:   totalStockValue,
		FinancialKPIs:     financialKPIs, // NOVO
	}

	data := getFlashes(c)
	data["title"] = "Dashboard de Monitoramento"
	data["UserRole"] = session.Get("userRole")
	data["UserName"] = session.Get("userName")
	data["ActivePage"] = "monitoring"
	data["DashboardData"] = dashboardData
	data["CurrentPeriod"] = period
	
	c.HTML(http.StatusOK, "monitoring_dashboard.html", data)
}
