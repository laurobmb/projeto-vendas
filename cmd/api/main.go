package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"projeto-vendas/internal/handlers"
	"projeto-vendas/internal/storage"
)

func main() {
	// 1. Inicializa a camada de armazenamento (storage)
	storageLayer, err := storage.NewStorage()
	if err != nil {
		log.Fatalf("Falha ao inicializar a camada de armazenamento: %v", err)
	}
	defer storageLayer.Dbpool.Close()

	// 2. Inicializa a camada de handlers, injetando a dependência do storage
	h := handlers.NewHandler(storageLayer)

	// 3. Configura o servidor Gin
	router := gin.Default()
	store := cookie.NewStore([]byte("super-secret-key"))
	router.Use(sessions.Sessions("mysession", store))
	router.StaticFS("/static", http.Dir("web/static"))

	// ATUALIZADO: Adicionada a função 'json' ao mapa de funções do template.
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
	stockRoutes.Use(AuthRequired("estoquista", "admin"))
	{
		stockRoutes.GET("/dashboard", h.ShowEstoquistaDashboard)
		stockRoutes.POST("/add", h.HandleAddStockItem)
	}
	
	salesRoutes := router.Group("/vendas")
	salesRoutes.Use(AuthRequired("vendedor", "admin"))
	{
		salesRoutes.GET("/terminal", h.ShowSalesTerminalPage)
	}

	adminRoutes := router.Group("/admin")
	adminRoutes.Use(AuthRequired("admin")) 
	{
		adminRoutes.GET("/dashboard", h.ShowAdminDashboard)
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
		adminRoutes.POST("/stock/update", h.HandleUpdateStock)
		adminRoutes.POST("/stock/add", h.HandleAddStockItem)
		adminRoutes.GET("/api/products/:id/stock", h.HandleGetProductStock)
		adminRoutes.POST("/api/stock/adjust", h.HandleAdjustStock)
	}

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

// Middleware de autenticação que aceita múltiplos cargos.
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
