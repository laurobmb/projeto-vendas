# Artigo Técnico: Construindo um Assistente de IA com "Tool Calling"

A integração de Inteligência Artificial em aplicações web evoluiu para além de simples chatbots que respondem a perguntas genéricas. A verdadeira revolução está em criar assistentes que não apenas conversam, mas que também **executam ações** e interagem com os dados de um sistema de forma segura e contextual.

Neste artigo, vamos explorar a arquitetura e as técnicas de IA implementadas no nosso Sistema de Gestão de Vendas, focando em como transformámos um Large Language Model (LLM) num assistente de negócios funcional para diferentes perfis de utilizador.

## Arquitetura: O Padrão de "Proxy" Seguro

A primeira e mais importante decisão de arquitetura foi **nunca expor chaves de API ou lógica de negócio sensível no frontend**. Colocar uma chave de API da Gemini diretamente no JavaScript seria uma falha de segurança grave, permitindo que qualquer pessoa a copiasse e a usasse em seu nome.

Para resolver isto, implementámos o **padrão de "proxy" no backend**:

1.  **Frontend (`chat.js`):** O nosso widget de chat não fala diretamente com a API da IA (seja Gemini ou Ollama). Em vez disso, ele envia todos os pedidos para um único endpoint seguro na nossa própria API em Go: `/api/chat`.
2.  **Backend (Go - `handlers.go`):** O nosso handler `HandleAIChat` atua como um intermediário. Ele recebe o pedido do frontend, lê a chave de API e o nome do modelo de forma segura a partir das variáveis de ambiente (`config.env`), faz a chamada para a API externa da IA e, finalmente, devolve apenas a resposta para o frontend.

Este padrão oferece três vantagens cruciais:
* **Segurança:** A chave de API nunca sai do nosso servidor.
* **Controlo:** Podemos adicionar lógica de validação, logging ou caching no nosso backend antes de contactar a IA.
* **Abstração:** O frontend não precisa de saber qual é o provedor de IA que estamos a usar.

## A Técnica Central: "Tool Calling" (Chamada de Ferramenta)

O "coração" da nossa implementação é a técnica de **Tool Calling**. Em vez de apenas pedir à IA para conversar, nós damos-lhe um conjunto de "ferramentas" (funções) que ela pode usar para obter informações do nosso sistema.

O fluxo de uma interação com Tool Calling acontece em dois passos principais:

1.  **Passo de "Pensamento":** O utilizador faz uma pergunta (ex: "quais foram as vendas do mês?"). A IA, em vez de tentar adivinhar a resposta, analisa as ferramentas que tem disponíveis e responde com uma instrução, como: "Por favor, execute a ferramenta `getSalesSummary`".
2.  **Passo de "Resposta":** O nosso código recebe esta instrução, executa a função correspondente (chamando a nossa API em Go), obtém os dados reais e envia-os de volta para a IA, dizendo: "Aqui está o resultado da ferramenta `getSalesSummary`. Agora, por favor, formule uma resposta amigável para o utilizador."

## Flexibilidade de Modelos: Gemini vs. Ollama

Uma das grandes vantagens da nossa arquitetura é a flexibilidade para escolher o motor de IA. No ficheiro `chat.js`, uma única variável de configuração controla qual serviço é utilizado:

```javascript
const AI_PROVIDER = "gemini"; // Mude para "ollama" para usar o seu modelo local
```

Isto permite-nos alternar entre um modelo de nuvem poderoso como o Gemini e um modelo local como o Llama 3 (através do Ollama) sem alterar a lógica principal da aplicação.

### 1\. Implementação com Google Gemini (Tool Calling Nativo)

Os modelos mais recentes da Gemini têm um suporte nativo e estruturado para Tool Calling, o que torna a implementação mais elegante e fiável.

  * **Definição das Ferramentas:** No `chat.js`, definimos as nossas ferramentas num formato JSON que a API da Gemini entende. A `description` é a parte mais importante, pois é o que a IA usa para decidir qual ferramenta chamar.

    ```javascript
    const geminiTools = [{
        functionDeclarations: [
            {
                name: "getTopSellers",
                description: "Obtém o ranking dos 3 melhores vendedores do mês atual com base no valor total de vendas."
            },
            // ... outras ferramentas
        ]
    }];
    ```

  * **Interpretação da Resposta:** A resposta da API da Gemini vem com um campo `functionCall` bem definido. O nosso código simplesmente verifica se este campo existe e, se existir, executa a função correspondente.

    ```javascript
    const part = result.candidates[0].content.parts[0];
    if (part.functionCall) {
        const { name, args } = part.functionCall;
        // ... executa a ferramenta 'name' com os 'args' ...
    }
    ```

### 2\. Implementação com Ollama (Tool Calling Simulado)

Usar modelos que correm localmente através de ferramentas como o **Ollama** oferece benefícios significativos em termos de privacidade de dados e custos. Ao executar um comando como `ollama run llama3`, o Ollama expõe uma API REST no endereço `http://localhost:11434`.

Como modelos open-source não têm um suporte nativo para Tool Calling, nós **simulamos** este comportamento através de **Prompt Engineering**.

  * **O "Prompt Mágico":** No `chat.js`, criámos um `systemPrompt` muito específico que instrui o modelo a comportar-se como a API da Gemini. A instrução chave é:

    ```
    Quando o utilizador pedir o ranking de vendedores, responda APENAS com o JSON: {"functionCall": "getTopSellers"}.
    ```

    Ao adicionar `format: "json"` ao pedido para o Ollama, aumentamos a probabilidade de o modelo seguir a instrução à risca.

  * **Interpretação da Resposta:** Como a resposta do Ollama é apenas texto, o nosso código precisa de tentar interpretá-la como um JSON.

    ```javascript
    const aiText = result.response.trim();
    try {
        const parsedResponse = JSON.parse(aiText);
        if (parsedResponse.functionCall === 'getTopSellers') {
            // ... executa a ferramenta ...
        }
    } catch (e) {
        // Se não for um JSON, é uma resposta de texto normal.
        return aiText;
    }
    ```

## Conclusão

Ao combinar o padrão de "proxy" seguro com a técnica de "Tool Calling" (tanto nativa como simulada), conseguimos criar um assistente de IA que é muito mais do que um simples chatbot. Ele é um verdadeiro copiloto para os utilizadores do sistema, capaz de aceder a dados em tempo real e executar ações de forma segura e controlada.

Esta arquitetura não só é poderosa, como também é extensível. Adicionar novas capacidades à nossa IA agora é tão simples como:

1.  Criar um novo endpoint na nossa API em Go.

2.  "Ensinar" a IA sobre a nova ferramenta, atualizando o `systemPrompt`.

3.  Adicionar um novo `if` ao nosso "router" de funções no `chat.js`.

Este é o futuro das aplicações web inteligentes: sistemas onde a IA e a lógica de negócio colaboram para criar experiências de utilizador mais ricas e eficientes.

