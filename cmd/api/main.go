package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"projeto-vendas/internal/handlers"
	"projeto-vendas/internal/storage"
)

func main() {
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
	})
	router.LoadHTMLGlob("web/templates/*.html")

	// --- Rotas Públicas ---
	router.GET("/login", h.ShowLoginPage)
	router.POST("/login", h.HandleLogin)
	router.GET("/logout", h.HandleLogout)


	stockRoutes := router.Group("/estoque")
	stockRoutes.Use(AuthRequired("estoquista", "admin")) // Permite acesso a estoquistas e admins
	{
		stockRoutes.GET("/dashboard", h.ShowEstoquistaDashboard)
		stockRoutes.POST("/add", h.HandleAddStockItem) // Reutiliza o handler de adicionar stock
	}
	// --- Rotas Protegidas ---
	// Grupo de rotas para o Vendedor
	salesRoutes := router.Group("/vendas")
	// CORREÇÃO: Permitir que tanto "vendedor" como "admin" acedam ao terminal.
	salesRoutes.Use(AuthRequired("vendedor", "admin"))
	{
		salesRoutes.GET("/terminal", h.ShowSalesTerminalPage)
	}

	// Grupo de rotas para o Admin
	adminRoutes := router.Group("/admin")
	adminRoutes.Use(AuthRequired("admin"))
	{
		adminRoutes.GET("/dashboard", h.ShowAdminDashboard)
		adminRoutes.GET("/stock", h.ShowStockManagementPage)
		adminRoutes.GET("/sales", h.ShowSalesReportPage) // NOVA ROTA
		adminRoutes.POST("/users/add", h.HandleAddUser)
		adminRoutes.POST("/users/delete/:id", h.HandleDeleteUser)
		adminRoutes.POST("/users/edit/:id", h.HandleEditUser) // NOVA ROTA
		adminRoutes.POST("/products/add", h.HandleAddProduct)
		adminRoutes.POST("/products/delete/:id", h.HandleDeleteProduct)
		adminRoutes.POST("/stock/update", h.HandleUpdateStock)
		adminRoutes.POST("/stock/add", h.HandleAddStockItem)
		adminRoutes.GET("/api/products/:id/stock", h.HandleGetProductStock)
		adminRoutes.POST("/api/stock/adjust", h.HandleAdjustStock)
	}

	// API para o terminal de vendas (protegida para vendedores e admins)
	apiRoutes := router.Group("/api")
	apiRoutes.Use(AuthRequired("vendedor", "admin"))
	{
		apiRoutes.GET("/products/search", h.HandleSearchProductsForSale)
		apiRoutes.POST("/sales", h.HandleRegisterSale)
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

// ATUALIZADO: Middleware agora renderiza uma página de erro.
func AuthRequired(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userRole, ok := session.Get("userRole").(string)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		
		for _, role := range requiredRoles {
			if userRole == role {
				c.Next()
				return
			}
		}
		
		log.Printf("Acesso negado para o utilizador com cargo '%s'. Rota requer um dos seguintes cargos: %v", userRole, requiredRoles)
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":        "Acesso Negado",
			"StatusCode":   http.StatusForbidden,
			"ErrorMessage": "Acesso Negado",
		})
		c.Abort()
	}
}