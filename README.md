# Sistema de Gestão de Vendas e Stock com Assistente de IA

Este é um sistema web completo para a gestão de um negócio de retalho com múltiplas filiais. A aplicação foi desenvolvida em Go (Golang), utiliza PostgreSQL como base de dados e integra um assistente de Inteligência Artificial para análise de dados em tempo real.

## Funcionalidades Principais

* **Autenticação por Cargos:** Sistema de login seguro que direciona os utilizadores para painéis específicos:
    * **Painel de Administrador:** Visão completa do negócio com gestão de utilizadores, catálogo de produtos, stock por filial, dados da empresa, sócios e relatórios de vendas.
    * **Painel de Estoquista:** Interface focada na gestão de stock da sua filial, permitindo a adição de novos produtos e a entrada de stock para itens existentes.
    * **Terminal de Vendas (PDV):** Ponto de Venda rápido e interativo para os vendedores, com busca de produtos e registo de transações.
* **Dashboard de Monitoramento (Admin):** Uma página de gestão visual com KPIs (faturamento, transações, ticket médio), um gráfico de vendas diárias por filial e uma lista de alertas de stock baixo.
* **Assistente de IA (Chatbot):** Um chat flutuante disponível para todos os perfis, capaz de responder a perguntas em linguagem natural e executar ações, como:
    * Analisar o faturamento por filial.
    * Gerar um ranking dos melhores vendedores.
    * Listar produtos com stock baixo.
    * Filtrar o catálogo de produtos por categoria e preço.

## Arquitetura da Inteligência Artificial

A integração da IA foi desenhada com foco em segurança, controlo e flexibilidade, utilizando a técnica de **"Tool Calling"**.

### Padrão de "Proxy" Seguro

A chave de API (seja da Gemini ou de outro serviço) **nunca é exposta no frontend**. O fluxo de comunicação é o seguinte:

1.  **Frontend (`chat.js`):** O chat envia a pergunta do utilizador para um endpoint seguro na nossa própria API em Go (`/api/chat`).
2.  **Backend (Go - `handlers.go`):** O nosso servidor atua como um intermediário. Ele recebe o pedido, lê a chave de API de forma segura a partir das variáveis de ambiente, faz a chamada para a API externa da IA e devolve apenas a resposta final para o frontend.

### Técnica Central: "Tool Calling"

Em vez de apenas conversar, demos à IA um conjunto de "ferramentas" (funções) que ela pode usar para obter informações do nosso sistema.

1.  **Passo de "Pensamento":** O utilizador pergunta "qual filial vendeu mais este mês?". A IA, em vez de adivinhar, analisa as suas ferramentas e responde com uma instrução, como: `{"functionCall": "getTopBillingBranch", "period": "month"}`.
2.  **Passo de "Resposta":** O nosso `chat.js` recebe esta instrução, executa a função correspondente (chamando a nossa API em Go), obtém os dados reais e envia-os de volta para a IA, dizendo: "Aqui está o resultado. Agora, formule uma resposta amigável para o utilizador."

#### Implementação com Google Gemini (Nativo) vs. Ollama (Simulado)

* **Google Gemini:** Usamos o suporte nativo da Gemini para "Tool Calling", definindo as nossas ferramentas e os seus parâmetros de forma estruturada. A API da Gemini devolve um objeto `functionCall` claro, o que torna a implementação mais robusta.
* **Ollama (Llama 3):** Como modelos locais não têm esta funcionalidade nativa, nós a **simulamos** através de *Prompt Engineering*. Instruímos o modelo no `systemPrompt` a devolver um JSON com uma estrutura específica sempre que a pergunta do utilizador corresponder a uma das nossas ferramentas.

## Relatório Visual dos Testes de Frontend

<h3 align="center">Fluxo Completo da Aplicação</h3>
<table width="100%" border="1" style="border-collapse: collapse; margin: auto;">
<thead>
<tr style="background-color: #f2f2f2;">
<th style="padding: 10px; text-align: center;">Passo do Teste</th>
<th style="padding: 10px; text-align: center;">Passo Seguinte</th>
</tr>
</thead>
<tbody>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014901_00_tela_login_preenchida.png" alt="Tela de login preenchida" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014901_01_admin_dashboard.png" alt="Dashboard do Admin" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014902_02_admin_modal_adicionar_user.png" alt="Modal para adicionar utilizador" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014903_03_admin_user_adicionado.png" alt="Utilizador adicionado" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014904_04_admin_user_removido.png" alt="Utilizador removido" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014904_05_admin_modal_adicionar_produto.png" alt="Modal para adicionar produto" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014905_06_admin_produto_adicionado.png" alt="Produto adicionado" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014906_09_vendedor_terminal_vazio.png" alt="Terminal de vendas do vendedor (vazio)" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014908_10_vendedor_item_no_carrinho.png" alt="Item adicionado ao carrinho" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014909_11_vendedor_venda_finalizada.png" alt="Venda finalizada" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014910_12_estoquista_dashboard.png" alt="Dashboard do estoquista" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014910_13_estoquista_modal_adicionar_existente.png" alt="Modal para adicionar stock a produto existente" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014911_14_estoquista_stock_adicionado_sucesso.png" alt="Mensagem de sucesso ao adicionar stock" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014912_15_estoquista_modal_criar_novo.png" alt="Modal para criar novo produto" width="100%"></td>
</tr>
<tr>
<td style="padding: 10px;"><img src="photos/20250811-014912_16_estoquista_novo_produto_criado_sucesso.png" alt="Mensagem de sucesso ao criar novo produto" width="100%"></td>
<td style="padding: 10px;"><img src="photos/20250811-014913_17_admin_dashboard_monitoramento.png" alt="Dashboard de Monitoramento do Admin" width="100%"></td>
</tr>
</tbody>
</table>

## Tecnologias Utilizadas

* **Backend:** Go (Golang)
* **Framework Web:** Gin Gonic
* **Base de Dados:** PostgreSQL
* **Frontend:** HTML5, Tailwind CSS, JavaScript, Chart.js
* **Inteligência Artificial:** Google Gemini / Ollama (Llama 3)
* **Testes:**
    * **Backend:** Pacote `testing` nativo do Go.
    * **Frontend (E2E):** Python com Selenium.

## Configuração e Execução

### 1. Pré-requisitos

* Go (versão 1.18 ou superior)
* PostgreSQL
* Python 3 (para os testes de frontend)
* Google Chrome e ChromeDriver (para os testes de frontend)

### 2. Configurar a Base de Dados

É recomendado usar um container para a base de dados para garantir um ambiente limpo e isolado.

```bash
# Exemplo com Podman (ou Docker)
podman run --rm --name postgres-vendas -e POSTGRES_PASSWORD=1q2w3e -e POSTGRES_USER=me -p 5432:5432 -d postgres:latest