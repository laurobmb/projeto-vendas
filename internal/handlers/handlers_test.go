package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"projeto-vendas/internal/models"
	"projeto-vendas/internal/storage"
)

// --- Mock da Camada de Storage ---

type mockStorage struct{}

var _ storage.Store = (*mockStorage)(nil)

func (m *mockStorage) GetUserByEmail(email string) (*models.User, error) {
	if email == "admin@teste.com" || email == "vendedor@teste.com" {
		cargo := "admin"
		if email == "vendedor@teste.com" {
			cargo = "vendedor"
		}
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)
		return &models.User{
			ID:        uuid.New(),
			Nome:      "Utilizador Teste",
			Email:     email,
			Cargo:     cargo,
			SenhaHash: string(hashedPassword),
			FilialID:  func() *uuid.UUID { u := uuid.New(); return &u }(),
		}, nil
	}
	return nil, errors.New("utilizador não encontrado")
}

func (m *mockStorage) SearchProductsForSale(query string, filialID uuid.UUID) ([]models.Product, error) {
	if query == "ProdutoExistente" {
		return []models.Product{
			{ID: uuid.New(), Nome: "Produto Existente", CodigoBarras: "123", PrecoSugerido: 10.0},
		}, nil
	}
	return []models.Product{}, nil
}

func (m *mockStorage) RegisterSale(sale models.Venda, items []models.ItemVenda) error {
	if len(items) == 0 {
		return errors.New("nenhum item na venda")
	}
	return nil
}

// Implementações vazias para as outras funções da interface.
func (m *mockStorage) GetFilialByID(id string) (*models.Filial, error) { return &models.Filial{ID: uuid.New(), Nome: "Filial Teste"}, nil }
func (m *mockStorage) CountUsers() (int, error)                       { return 1, nil }
func (m *mockStorage) GetUsersPaginated(limit, offset int) ([]models.User, error) { return []models.User{}, nil }
func (m *mockStorage) CountProducts(searchQuery string) (int, error) { return 0, nil }
func (m *mockStorage) GetProductsPaginatedAndFiltered(searchQuery string, limit, offset int) ([]models.Product, error) { return []models.Product{}, nil }
func (m *mockStorage) GetAllFiliais() ([]models.Filial, error) { return []models.Filial{}, nil }
func (m *mockStorage) UpdateUser(userID string, user models.User, newPassword string) error { return nil }
func (m *mockStorage) CountSales(filialID string) (int, error) { return 0, nil }
func (m *mockStorage) GetSalesPaginated(filialID string, limit, offset int) ([]models.SaleReportItem, error) { return []models.SaleReportItem{}, nil }
func (m *mockStorage) CreateProductWithInitialStock(product models.Product, filialID string, quantity int) error { return nil }
func (m *mockStorage) AddStockItem(productID, filialID string, quantity int) error { return nil }
func (m *mockStorage) GetAllProductsSimple() ([]models.Product, error) { return []models.Product{}, nil }
func (m *mockStorage) CountStockItems(filialID, searchQuery string) (int, error) { return 0, nil }
func (m *mockStorage) GetStockItemsPaginated(filialID, searchQuery string, limit, offset int) ([]models.StockViewItem, error) { return []models.StockViewItem{}, nil }
func (m *mockStorage) UpdateStockQuantity(productID, filialID string, newQuantity int) error { return nil }
func (m *mockStorage) AddUser(user models.User, password string) error { return nil }
func (m *mockStorage) AddProduct(product models.Product) error { return nil }
func (m *mockStorage) UpdateSocio(socioID string, socio models.Socio) error { return nil }
func (m *mockStorage) GetEmpresa() (*models.Empresa, error) { return &models.Empresa{}, nil }
func (m *mockStorage) UpsertEmpresa(empresa models.Empresa) error { return nil }
func (m *mockStorage) GetSocios(empresaID uuid.UUID) ([]models.Socio, error) { return []models.Socio{}, nil }
func (m *mockStorage) AddSocio(socio models.Socio) error { return nil }
func (m *mockStorage) DeleteSocioByID(id string) error { return nil }
func (m *mockStorage) DeleteUserByID(id string) error { return nil }
func (m *mockStorage) DeleteProductByID(id string) error { return nil }
func (m *mockStorage) GetProductStockByFilial(productID string) ([]models.StockDetail, error) { return []models.StockDetail{}, nil }
func (m *mockStorage) AdjustStockQuantity(productID, filialID string, quantityToRemove int) error { return nil }


// --- Fim do Mock ---

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	store := cookie.NewStore([]byte("test-secret"))
	router.Use(sessions.Sessions("mysession", store))

	h := NewHandler(&mockStorage{})

	router.SetFuncMap(map[string]interface{}{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 { return nil, errors.New("invalid dict call") }
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok { return nil, errors.New("dict keys must be strings") }
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"json": func(v interface{}) (template.JS, error) {
			a, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(a), nil
		},
	})

	router.LoadHTMLGlob("../../web/templates/*.html")

	router.POST("/login", h.HandleLogin)
	
	adminRoutes := router.Group("/admin")
	adminRoutes.Use(h.AuthRequired("admin"))
	{
		adminRoutes.GET("/dashboard", h.ShowAdminDashboard)
	}

	apiRoutes := router.Group("/api")
	apiRoutes.Use(h.AuthRequired("vendedor", "admin"))
	{
		apiRoutes.GET("/products/search", h.HandleSearchProductsForSale)
		apiRoutes.POST("/sales", h.HandleRegisterSale)
	}

	return router
}

// loginAs é uma função auxiliar para simular um login e obter o cookie de sessão.
func loginAs(router *gin.Engine, email, password string) (*http.Cookie, error) {
	form := url.Values{}
	form.Add("email", email)
	form.Add("password", password)

	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		return nil, errors.New("login falhou")
	}

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		return nil, errors.New("nenhum cookie de sessão encontrado")
	}

	return cookies[0], nil
}

func TestLoginHandler(t *testing.T) {
	router := setupTestRouter()

	t.Run("Deve fazer login com sucesso com credenciais válidas", func(t *testing.T) {
		cookie, err := loginAs(router, "admin@teste.com", "senha123")
		assert.NoError(t, err)
		assert.NotEmpty(t, cookie)
	})

	t.Run("Deve falhar o login com senha incorreta", func(t *testing.T) {
		_, err := loginAs(router, "admin@teste.com", "senha_errada")
		assert.Error(t, err)
	})
}

func TestAdminDashboardAuth(t *testing.T) {
	router := setupTestRouter()

	t.Run("Deve redirecionar para o login se não estiver autenticado", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/dashboard", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login", w.Header().Get("Location"))
	})
}

func TestAPIHandlers(t *testing.T) {
	router := setupTestRouter()

	sessionCookie, err := loginAs(router, "vendedor@teste.com", "senha123")
	assert.NoError(t, err)
	assert.NotNil(t, sessionCookie)

	t.Run("Deve buscar produtos com sucesso via API", func(t *testing.T) {
		filialID := uuid.New().String()
		req, _ := http.NewRequest("GET", "/api/products/search?q=ProdutoExistente&filial_id="+filialID, nil)
		req.AddCookie(sessionCookie)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var products []models.Product
		err := json.Unmarshal(w.Body.Bytes(), &products)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, "Produto Existente", products[0].Nome)
	})

	t.Run("Deve falhar a busca de produtos sem autenticação", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/products/search?q=ProdutoExistente&filial_id="+uuid.NewString(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// CORREÇÃO: O teste agora espera 401 Unauthorized, que é a resposta correta para um pedido de API não autenticado.
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
