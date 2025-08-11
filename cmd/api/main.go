package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"projeto-vendas/internal/handlers"
	"projeto-vendas/internal/storage"
)

func loadEnv() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Não foi possível encontrar o caminho do ficheiro main.")
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

	// Carrega o ficheiro config.env a partir da raiz do projeto
	envPath := filepath.Join(dir, "config.env") // Volta para a raiz do projeto
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Aviso: Não foi possível carregar o ficheiro config.env de %s: %v", envPath, err)
	}
}

func main() {

	loadEnv()

	// --- TROUBLESHOOTING: Exibe a chave de API encontrada ---
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey != "" {
		log.Printf("✅ Chave de API da Gemini carregada com sucesso. (Final: ...%s)", apiKey[len(apiKey)-4:])
	} else {
		log.Println("🚨 AVISO: A variável de ambiente GEMINI_API_KEY não foi encontrada.")
	}
	// --- Fim do Troubleshooting ---

	storageLayer, err := storage.NewStorage()
	if err != nil {
		log.Fatalf("Falha ao inicializar a camada de armazenamento: %v", err)
	}
	defer storageLayer.Dbpool.Close()

	h := handlers.NewHandler(storageLayer)

	router := gin.Default()
	store := cookie.NewStore([]byte("super-secret-key"))
	router.Use(sessions.Sessions("mysession", store))
	router.StaticFS("/static", http.Dir("web/static"))

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
	router.LoadHTMLGlob("web/templates/*.html")

	// --- Rotas Públicas ---
	router.GET("/login", h.ShowLoginPage)
	router.POST("/login", h.HandleLogin)
	router.GET("/logout", h.HandleLogout)

	// --- Rotas Protegidas ---
	stockRoutes := router.Group("/estoque")
	stockRoutes.Use(h.AuthRequired("estoquista", "admin")) // CORREÇÃO
	{
		stockRoutes.GET("/dashboard", h.ShowEstoquistaDashboard)
		stockRoutes.POST("/add", h.HandleAddStockItem)
	}
	
	salesRoutes := router.Group("/vendas")
	salesRoutes.Use(h.AuthRequired("vendedor", "admin")) // CORREÇÃO
	{
		salesRoutes.GET("/terminal", h.ShowSalesTerminalPage)
	}

	adminRoutes := router.Group("/admin")
	adminRoutes.Use(h.AuthRequired("admin")) // CORREÇÃO
	{
		adminRoutes.GET("/dashboard", h.ShowAdminDashboard)
		adminRoutes.GET("/monitoring", h.ShowMonitoringDashboard)
		adminRoutes.GET("/stock", h.ShowStockManagementPage)
		adminRoutes.GET("/sales", h.ShowSalesReportPage)
		adminRoutes.GET("/empresa", h.ShowEmpresaPage)
		adminRoutes.POST("/empresa/update", h.HandleUpdateEmpresa)
		adminRoutes.POST("/socios/add", h.HandleAddSocio)
		adminRoutes.POST("/socios/delete/:id", h.HandleDeleteSocio)
		adminRoutes.POST("/socios/edit/:id", h.HandleEditSocio)
		adminRoutes.POST("/users/add", h.HandleAddUser)
		adminRoutes.POST("/users/delete/:id", h.HandleDeleteUser)
		adminRoutes.POST("/users/edit/:id", h.HandleEditUser)
		adminRoutes.POST("/products/add", h.HandleAddProduct)
		adminRoutes.POST("/products/delete/:id", h.HandleDeleteProduct)
		adminRoutes.POST("/products/edit/:id", h.HandleEditProduct)
		adminRoutes.POST("/stock/update", h.HandleUpdateStock)
		adminRoutes.POST("/stock/add", h.HandleAddStockItem)
		adminRoutes.GET("/api/products/:id/stock", h.HandleGetProductStock)
		adminRoutes.POST("/api/stock/set", h.HandleSetStock)
	}

	salesApiRoutes := router.Group("/api/sales")
	salesApiRoutes.Use(h.AuthRequired("admin", "vendedor"))
	{
		salesApiRoutes.GET("/summary", h.HandleGetSalesSummary)
		salesApiRoutes.GET("/topsellers", h.HandleGetTopSellers)
		salesApiRoutes.GET("/topbilling", h.HandleGetTopBillingBranch)
		salesApiRoutes.GET("/branchsummary", h.HandleGetSalesSummaryByBranch)
	}

	apiRoutes := router.Group("/api")
	apiRoutes.Use(h.AuthRequired("vendedor", "admin", "estoquista")) // CORREÇÃO
	{
		apiRoutes.GET("/products/search", h.HandleSearchProductsForSale)
		apiRoutes.POST("/sales", h.HandleRegisterSale)
		apiRoutes.GET("/products/filter", h.HandleFilterProducts)
		apiRoutes.GET("/stock/low", h.HandleGetLowStockProducts) // NOVA ROTA
		apiRoutes.POST("/chat", h.HandleAIChat)
	}

	// Redirecionamento principal
	router.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		role, ok := session.Get("userRole").(string)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		switch role {
		case "admin":
			c.Redirect(http.StatusFound, "/admin/dashboard")
		case "vendedor":
			c.Redirect(http.StatusFound, "/vendas/terminal")
		case "estoquista":
			c.Redirect(http.StatusFound, "/estoque/dashboard")
		default:
			c.Redirect(http.StatusFound, "/login")
		}
	})

	log.Println("🚀 Servidor iniciado em http://localhost:8080")
	router.Run(":8080")
}
