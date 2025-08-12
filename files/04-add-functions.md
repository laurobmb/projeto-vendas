# How-To: Adicionar uma Nova Ferramenta ao Assistente de IA

Este guia descreve o processo completo para adicionar uma nova capacidade ao nosso assistente de IA, desde a consulta à base de dados até à resposta final no chat. Usaremos como exemplo a criação de uma ferramenta para "encontrar o produto mais caro do catálogo".

O processo é dividido em duas grandes etapas: **Backend (criar a ferramenta)** e **Frontend (ensinar a IA a usá-la)**.

---

### Passo 1: Backend (Go) - Criar a Ferramenta

Primeiro, precisamos de criar a lógica que busca a informação no nosso banco de dados e a expõe através de uma API.

#### 1a. `internal/models/models.go`
Crie uma `struct` para representar os dados que a sua nova função irá devolver.

```go
// models.go
type MostExpensiveProduct struct {
    Nome  string  `json:"nome"`
    Preco float64 `json:"preco"`
}
````

#### 1b. `internal/storage/storage.go`

Adicione a nova função à `interface Store` e, em seguida, implemente-a. Esta função conterá a sua consulta SQL.

```go
// storage.go

// Adicionar à interface Store
type Store interface {
    // ...
    GetMostExpensiveProduct() (*models.MostExpensiveProduct, error)
}

// Implementar a função
func (s *Storage) GetMostExpensiveProduct() (*models.MostExpensiveProduct, error) {
    var product models.MostExpensiveProduct
    sql := `
        SELECT nome, preco_sugerido 
        FROM produtos 
        ORDER BY preco_sugerido DESC 
        LIMIT 1;
    `
    err := s.Dbpool.QueryRow(context.Background(), sql).Scan(&product.Nome, &product.Preco)
    if err != nil {
        return nil, err
    }
    return &product, nil
}
```

#### 1c. `internal/handlers/handlers.go`

Crie um novo handler HTTP para expor a sua função de `storage` como um endpoint de API.

```go
// handlers.go
func (h *Handler) HandleGetMostExpensiveProduct(c *gin.Context) {
    product, err := h.Storage.GetMostExpensiveProduct()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao buscar produto."})
        return
    }
    c.JSON(http.StatusOK, product)
}
```

#### 1d. `cmd/api/main.go`

Finalmente, registe a nova rota para que a sua API a reconheça.

```go
// main.go
apiRoutes := router.Group("/api")
apiRoutes.Use(h.AuthRequired("vendedor", "admin", "estoquista"))
{
    // ... (outras rotas) ...
    apiRoutes.GET("/products/most-expensive", h.HandleGetMostExpensiveProduct) // NOVA ROTA
}
```

-----

### Passo 2: Frontend (JavaScript) - Ensinar a IA

Agora que o backend tem a "ferramenta", precisamos de "ensinar" o `chat.js` a usá-la.

#### 2a. `web/static/js/chat.js` - Objeto `tools`

Adicione uma nova função ao objeto `tools` que chama o seu novo endpoint de API.

```javascript
// chat.js
const tools = {
    // ... (outras ferramentas) ...
    async getMostExpensiveProduct() {
        return await fetch('/api/products/most-expensive').then(res => res.json());
    }
};
```

#### 2b. `web/static/js/chat.js` - Lógica da Gemini

Adicione a definição da nova ferramenta à lista `geminiTools`. A `description` é crucial, pois é o que a IA usa para decidir quando usar a ferramenta.

```javascript
// chat.js -> getGeminiResponse()
const geminiTools = [{
    functionDeclarations: [
        // ... (outras ferramentas) ...
        {
            name: "getMostExpensiveProduct",
            description: "Encontra o produto com o preço de venda mais alto em todo o catálogo."
        }
    ]
}];
```

#### 2c. `web/static/js/chat.js` - Roteamento da Gemini

Adicione um novo `else if` ao bloco `if (part.functionCall)` para lidar com a chamada da nova ferramenta.

```javascript
// chat.js -> getGeminiResponse()
if (part.functionCall) {
    const { name, args } = part.functionCall;
    let toolResult;

    // ... (outros if/else if) ...
    else if (name === 'getMostExpensiveProduct') {
        toolResult = await tools.getMostExpensiveProduct();
    }
    // ...
}
```

#### 2d. `web/static/js/chat.js` - Lógica do Ollama

Atualize o `systemPrompt` para incluir a nova ferramenta na lista e adicione uma nova instrução de como a chamar.

```javascript
// chat.js -> getOllamaResponse()
let systemPrompt = `
    ...
    Ferramentas disponíveis:
    // ...
    9. getMostExpensiveProduct()
`;

systemPrompt += `
    Instruções de chamada de função:
    // ...
    - Para getMostExpensiveProduct, responda APENAS com: {"functionCall": "getMostExpensiveProduct"}
    Para qualquer outra pergunta, responda normalmente.
`;
```

#### 2e. `web/static/js/chat.js` - Roteamento do Ollama

Adicione um novo `case` ao `switch` para lidar com a chamada simulada e formatar o `dataPrompt` que será enviado de volta para a IA.

```javascript
// chat.js -> getOllamaResponse()
switch (parsedResponse.functionCall) {
    // ... (outros cases) ...
    case 'getMostExpensiveProduct': {
        const product = await tools.getMostExpensiveProduct();
        if (!product) {
            dataPrompt = "Não foi possível encontrar o produto mais caro.";
        } else {
            dataPrompt = `O produto mais caro do catálogo é '${product.nome}', custando R$ ${product.preco.toFixed(2)}.`;
        }
        toolCalled = true;
        break;
    }
}
```

