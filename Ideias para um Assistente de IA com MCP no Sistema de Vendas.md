# Conceito: O Assistente Inteligente do Atacarejo

Imagine um pequeno widget de chat em todas as páginas. Este não é um chatbot comum; é um assistente que entende o contexto do utilizador (quem ele é, qual a sua filial) e tem acesso a um conjunto de ferramentas específicas para o ajudar, tudo isto através do **Model Context Protocol (MCP)**.

---

## Para o Administrador: O Analista de Negócios Pessoal

O administrador tem acesso a tudo, mas precisa de extrair informações de forma rápida. O assistente de IA transforma a análise de dados numa conversa.

### Exemplos de Interações

**Análise de Vendas:**
- "Qual filial teve o maior faturamento este mês?"
- "Mostra-me um resumo das vendas da última semana na Filial Centro."
- "Quem foi o vendedor com mais vendas ontem?"

**Análise de Stock:**
- "Qual é o valor total do meu stock em todas as filiais?"
- "Lista-me os 5 produtos com o stock mais baixo na Filial Zona Sul."
- "Há algum produto com stock zero em alguma filial?"

**Análise de Catálogo:**
- "Quais são os produtos da categoria 'Eletrónicos' com preço superior a 3000?"

**Como o MCP funciona aqui:**
O nosso Servidor MCP iria expor *ferramentas* ligadas à nossa API Go, como:
- `getSalesSummary(periodo, filial)`
- `getTotalStockValue(categoria)`
- `getLowStockProducts(filial, limite)`

A IA iria interpretar a pergunta do administrador e chamar a ferramenta correta com os parâmetros certos.

---

## Para o Estoquista: O Assistente de Inventário

O estoquista está focado na sua filial. O assistente de IA pode acelerar as suas tarefas diárias e reduzir erros.

### Exemplos de Interações

**Consulta Rápida:**
- "Qual é o stock atual do produto com código de barras 789123456?"
- "Encontra o produto 'Óleo de Soja Premium'." (A IA faria a busca na tabela)

**Ações Rápidas:**
- "Adiciona 150 unidades do 'Arroz Top S-Line' ao meu stock."
- "Cria um novo produto chamado 'Refrigerante Uva 2L', código 987654321, preço 7.50, e adiciona 300 unidades ao stock da minha filial."

**Alertas e Relatórios:**
- "Lista-me todos os produtos com menos de 50 unidades no meu stock."
- "Quais foram os últimos produtos que adicionei ao stock hoje?"

**Como o MCP funciona aqui:**
O assistente saberia automaticamente a filial do estoquista a partir da sua sessão de login.  
As ferramentas do MCP seriam:
- `getStockByBarcode(codigo, filial)`
- `addStock(produto, quantidade, filial)`
- `createNewProductWithStock(...)`

---

## Para o Vendedor: O Copiloto de Vendas

No terminal de vendas, a velocidade é essencial. O assistente de IA pode fornecer informações cruciais sem que o vendedor precise de sair da tela ou abrir outros menus.

### Exemplos de Interações

**Informação de Produto:**
- "Quais são os detalhes do 'Notebook Tech Pro'?" (A IA mostraria a descrição completa).
- "Este smartphone tem garantia?" (A IA poderia ser treinada para responder a perguntas frequentes).

**Consulta de Stock Cruzado (Cross-Selling):**
- "O cliente quer um 'Fone de Ouvido Bluetooth', mas não temos stock. Verifica se há na Filial Centro."

**Ações no Carrinho (Potencial para comandos de voz):**
- "Adiciona 5 unidades de 'Sabão em Pó Top' à venda."
- "Remove o último item que adicionei."

**Como o MCP funciona aqui:**
As ferramentas seriam:
- `getProductDetails(produto)`
- `checkStockInOtherFilial(produto, filial_destino)`
- Ferramentas que interagem com o JavaScript da página, como `addToCart(produto, quantidade)`

O MCP pode servir como uma ponte entre a linguagem natural e as ações na interface do utilizador.

---

## Próximos Passos

A melhor forma de começar é escolher a funcionalidade mais simples e de maior impacto.  
A análise de dados para o administrador é um excelente ponto de partida.

Podemos começar por construir:
1. Um novo endpoint na nossa API Go (ex: `/api/sales/summary`) que retorna dados de vendas.
2. Um Servidor MCP em Go ou Python com uma única ferramenta: `getSalesSummary`.
3. Uma interface de chat simples no painel do administrador que se conecta a este servidor.

Quando estiver pronto, podemos começar a construir esta primeira integração.
