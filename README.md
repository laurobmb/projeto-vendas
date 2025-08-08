Com certeza\! Aqui está o conteúdo do `README.md` em formato cru, pronto para ser copiado:

# Sistema de Gestão de Vendas e Stock

Este é um sistema web completo para a gestão de um negócio de retalho com múltiplas filiais. A aplicação foi desenvolvida em Go (Golang) e utiliza PostgreSQL como base de dados. O sistema oferece diferentes painéis e funcionalidades baseados no cargo do utilizador (Administrador, Estoquista, Vendedor).

## Funcionalidades Principais

* **Autenticação por Cargos:** Sistema de login seguro que direciona os utilizadores para painéis específicos:
    * **Painel de Administrador:** Visão completa do negócio com gestão de utilizadores, catálogo de produtos, stock por filial, dados da empresa, sócios e relatórios de vendas.
    * **Painel de Estoquista:** Interface focada na gestão de stock da sua filial, permitindo a adição de novos produtos e a entrada de stock para itens existentes.
    * **Terminal de Vendas (PDV):** Ponto de Venda rápido e interativo para os vendedores, com busca de produtos por nome ou código de barras e registo de transações em tempo real.
* **Gestão Multi-Filial:** O stock é gerido de forma granular por cada loja/filial.
* **Catálogo de Produtos Centralizado:** Um painel para gerir todos os produtos, incluindo nome, descrição, preço e código de barras.
* **Relatórios de Vendas:** Visualização de todas as vendas realizadas, com filtros por filial.
* **Gestão de Dados da Empresa:** Uma área para gerir os dados fiscais da empresa e os seus sócios, preparando o sistema para a futura emissão de notas fiscais.

## Tecnologias Utilizadas

* **Backend:** Go (Golang)
* **Framework Web:** Gin Gonic
* **Base de Dados:** PostgreSQL
* **Frontend:** HTML5, Tailwind CSS, JavaScript
* **Testes:**
    * **Backend:** Pacote `testing` nativo do Go.
    * **Frontend (E2E):** Python com Selenium.

## Pré-requisitos

* Go (versão 1.18 ou superior)
* PostgreSQL
* Python 3 (para os testes de frontend)
* Google Chrome e ChromeDriver (para os testes de frontend)

## Configuração do Ambiente

1.  **Clone o repositório:**
    ```bash
    git clone <url-do-seu-repositorio>
    cd projeto-vendas
    ```

2.  **Configure as Variáveis de Ambiente:**
    Crie um ficheiro chamado `config.env` na raiz do projeto com as suas credenciais da base de dados de desenvolvimento:
    ```env
    DB_HOST=localhost
    DB_USER=seu_usuario_pg
    DB_PASS=sua_senha_pg
    DB_NAME=wallmart
    ```

3.  **Instale as Dependências do Go:**
    ```bash
    go mod tidy
    ```

## Como Executar

### 1. Preparar a Base de Dados

Execute o nosso gestor de dados para criar a base de dados e todas as tabelas necessárias.

```bash
go run ./cmd/data_manager/main.go -init
````

### 2\. Popular com Dados de Teste (Opcional)

Para ter um ambiente com dados realistas (filiais, empresa, sócios e 2000 produtos), execute o script de população:

```bash
go run ./cmd/populando_banco/main.go
```

### 3\. Criar Utilizadores

Use a ferramenta de linha de comando para criar os seus utilizadores iniciais:

```bash
# Criar um Administrador
go run ./cmd/create_user/main.go -name="Admin" -email="admin@email.com" -password="senha123" -role="admin"

# Criar um Vendedor (precisa do ID de uma filial)
go run ./cmd/create_user/main.go -name="Vendedor" -email="vendedor@email.com" -password="senha123" -role="vendedor" -filialid="<id-da-filial>"
```

> **Dica:** Para obter os IDs das filiais, use o comando: `go run ./cmd/data_manager/main.go -list-filiais`

### 4\. Iniciar o Servidor Web

Com a base de dados pronta, inicie a aplicação principal:

```bash
go run ./cmd/api/main.go
```

A aplicação estará disponível em `http://localhost:8080`.

## Como Executar os Testes

### 1\. Testes de Backend (Go)

Crie um ficheiro `.env.test` na raiz do projeto com as credenciais para uma base de dados de teste (ela será criada e apagada automaticamente).

```env
DB_HOST=localhost
DB_USER=seu_usuario_pg
DB_PASS=sua_senha_pg
DB_NAME=wallmart_test
```

Execute o seguinte comando a partir da raiz do projeto:

```bash
go test ./... -v
```

### 2\. Testes de Frontend (Python + Selenium)

Certifique-se de que tem o `selenium` e o `psycopg2-binary` instalados:

```bash
pip install selenium psycopg2-binary
```

Com o servidor web a correr (`go run ./cmd/api/main.go`), abra um **novo terminal** e execute o script de teste:

```bash
python3 test_frontend.py
```

## Licença

Este projeto está licenciado sob a Licença MIT.

### Licença MIT

Copyright (c) 2025 Seu Nome ou Nome da Empresa

É concedida permissão, gratuitamente, a qualquer pessoa que obtenha uma cópia deste software e dos ficheiros de documentação associados (o "Software"), para negociar o Software sem restrições, incluindo, sem limitação, os direitos de usar, copiar, modificar, fundir, publicar, distribuir, sublicenciar e/ou vender cópias do Software, e para permitir que as pessoas a quem o Software é fornecido o façam, sujeito às seguintes condições:

O aviso de direitos de autor acima e este aviso de permissão devem ser incluídos em todas as cópias ou partes substanciais do Software.

O SOFTWARE É FORNECIDO "COMO ESTÁ", SEM GARANTIA DE QUALQUER TIPO, EXPRESSA OU IMPLÍCITA, INCLUINDO, MAS NÃO SE LIMITANDO A, GARANTIAS DE COMERCIALIZAÇÃO, ADEQUAÇÃO A UM DETERMINADO FIM E NÃO INFRAÇÃO. EM NENHUM CASO OS AUTORES OU DETENTORES DOS DIREITOS DE AUTOR SERÃO RESPONSÁVEIS POR QUALQUER RECLAMAÇÃO, DANOS OU OUTRA RESPONSABILIDADE, SEJA NUMA AÇÃO DE CONTRATO, DELITO OU OUTRA FORMA, DECORRENTE DE, FORA DE OU EM CONEXÃO COM O SOFTWARE OU O USO OU OUTRAS NEGOCIAÇÕES NO SOFTWARE.

