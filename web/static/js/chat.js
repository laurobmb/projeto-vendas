document.addEventListener('DOMContentLoaded', () => {
    // --- Configurações do Assistente de IA ---
    const AI_PROVIDER = "gemini"; // Mude para "ollama" para usar o seu modelo local

    // Configurações do Ollama (usadas apenas se AI_PROVIDER for "ollama")
    const OLLAMA_URL = "http://localhost:11434/api/generate";
    const OLLAMA_MODEL = "llama2";
    // -----------------------------------------

    const chatIcon = document.getElementById('chat-icon');
    const chatWindow = document.getElementById('chat-window');
    const closeChatBtn = document.getElementById('close-chat');
    const chatForm = document.getElementById('chat-form');
    const chatInput = document.getElementById('chat-input');
    const chatMessages = document.getElementById('chat-messages');

    const userRole = window.USER_ROLE || '';

    let chatHistory = [];

    if (!chatIcon) return;

    chatIcon.addEventListener('click', () => chatWindow.classList.toggle('hidden'));
    closeChatBtn.addEventListener('click', () => chatWindow.classList.add('hidden'));

    chatForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const userMessage = chatInput.value.trim();
        if (!userMessage) return;

        addMessage(userMessage, 'user');
        chatInput.value = '';
        chatInput.disabled = true;
        chatForm.querySelector('button').disabled = true;
        showThinkingIndicator();

        try {
            const aiResponse = await getAIResponse(userMessage);
            addMessage(aiResponse, 'ai');
        } catch (error) {
            console.error("Erro ao comunicar com a IA:", error);
            addMessage("Desculpe, ocorreu um erro ao tentar processar o seu pedido.", 'ai-error');
        } finally {
            removeThinkingIndicator();
            chatInput.disabled = false;
            chatForm.querySelector('button').disabled = false;
            chatInput.focus();
        }
    });

    function addMessage(text, sender) {
        const messageDiv = document.createElement('div');
        messageDiv.classList.add('mb-3', 'p-3', 'rounded-lg', 'max-w-xs', 'break-words');
        messageDiv.textContent = text;

        if (sender === 'user') {
            messageDiv.classList.add('bg-blue-500', 'text-white', 'ml-auto');
        } else {
            messageDiv.classList.add('bg-gray-200', 'text-gray-800', 'mr-auto');
            if (sender === 'ai-error') {
                messageDiv.classList.remove('bg-gray-200');
                messageDiv.classList.add('bg-red-200', 'text-red-800');
            }
        }
        chatMessages.appendChild(messageDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    function showThinkingIndicator() {
        const thinkingDiv = document.createElement('div');
        thinkingDiv.id = 'thinking-indicator';
        thinkingDiv.classList.add('mb-3', 'p-3', 'rounded-lg', 'max-w-xs', 'bg-gray-200', 'text-gray-500', 'italic');
        thinkingDiv.textContent = 'A pensar...';
        chatMessages.appendChild(thinkingDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }

    function removeThinkingIndicator() {
        const indicator = document.getElementById('thinking-indicator');
        if (indicator) {
            indicator.remove();
        }
    }

    // --- Lógica de Ferramentas e API ---

    const tools = {
        async getSalesSummary() {
            return await fetch('/api/sales/summary').then(res => res.json());
        },
        async filterProducts(category, min_price) {
            return await fetch(`/api/products/filter?category=${category}&min_price=${min_price}`).then(res => res.json());
        },
        async getTopSellers() {
            return await fetch('/api/sales/topsellers').then(res => res.json());
        },
        async getLowStockProducts(limit, filial) {
            let url = `/api/stock/low?limit=${limit || 5}`;
            if (filial) {
                url += `&filial=${encodeURIComponent(filial)}`;
            }
            return await fetch(url).then(res => res.json());
        },
        async getTopBillingBranch(period) {
            return await fetch(`/api/sales/topbilling?period=${period}`).then(res => res.json());
        },
        async getSalesSummaryByBranch(period, branch) {
            return await fetch(`/api/sales/branchsummary?period=${period}&branch=${encodeURIComponent(branch)}`).then(res => res.json());
        },
        async getTopSellerByPeriod(period) {
            return await fetch(`/api/sales/topsellers?period=${period}`).then(res => res.json());
        },
        async getProductDetails(identifier) {
            return await fetch(`/api/products/details?identifier=${identifier}`).then(res => res.json());
        }
    };

    // --- Roteador de IA ---

    async function getAIResponse(prompt) {
        chatHistory.push({ role: "user", parts: [{ text: prompt }] });

        if (AI_PROVIDER === 'gemini') {
            return await getGeminiResponse();
        } else {
            return await getOllamaResponse(prompt);
        }
    }

    // --- Lógica para o Gemini (com Tool Calling via Proxy) ---

    async function getGeminiResponse() {
        let geminiTools = [{
            functionDeclarations: [
                { name: "getSalesSummary", description: "Obtém um resumo geral de vendas por filial." },
                {
                    name: "filterProducts",
                    description: "Filtra produtos por categoria e preço mínimo.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            category: { type: "STRING", description: "A categoria do produto a ser pesquisada." },
                            min_price: { type: "NUMBER", description: "O preço mínimo para o filtro." }
                        },
                        required: ["category", "min_price"]
                    }
                },
                { name: "getTopSellers", description: "Obtém o ranking dos 3 melhores vendedores do mês atual." },
                {
                    name: "getLowStockProducts",
                    description: "Obtém uma lista de produtos com o stock mais baixo.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            limit: { type: "NUMBER", description: "O número de produtos a retornar. Padrão é 5." },
                            filial: { type: "STRING", description: "O nome da filial para filtrar. Se omitido, busca em todas as filiais." }
                        },
                        required: ["limit"]
                    }
                },
                {
                    name: "getProductDetails",
                    description: "Obtém todos os detalhes de um produto específico, usando o seu código de barras ou código CNAE como identificador.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            identifier: { type: "STRING", description: "O código de barras ou CNAE do produto." }
                        },
                        required: ["identifier"]
                    }
                }
            ]
        }];
        
        if (userRole === 'admin' || userRole === 'vendedor') {
            geminiTools[0].functionDeclarations.push(
                {
                    name: "getTopBillingBranch",
                    description: "Encontra a filial com o maior faturamento num determinado período.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            period: { type: "STRING", description: "O período a ser analisado. Valores válidos: 'day', 'week', 'month'." }
                        },
                        required: ["period"]
                    }
                },
                {
                    name: "getSalesSummaryByBranch",
                    description: "Obtém um resumo detalhado de vendas para uma filial específica num período.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            period: { type: "STRING", description: "O período a ser analisado. Valores válidos: 'day', 'week', 'month'." },
                            branch: { type: "STRING", description: "O nome exato da filial a ser analisada." }
                        },
                        required: ["period", "branch"]
                    }
                },
                {
                    name: "getTopSellerByPeriod",
                    description: "Encontra o vendedor com o maior valor total de vendas num período.",
                    parameters: {
                        type: "OBJECT",
                        properties: {
                            period: { type: "STRING", description: "O período a ser analisado. Valores válidos: 'day', 'week', 'month'." }
                        },
                        required: ["period"]
                    }
                }
            );
        }

        const payload = {
            contents: chatHistory,
            tools: geminiTools,
            systemInstruction: {
                role: "system",
                parts: [{ text: `
                    Você é um assistente de negócios. Se o utilizador perguntar "quem é você?", apresente-se e descreva as suas capacidades com base nas ferramentas que conhece.
                    As suas ferramentas são: getSalesSummary, filterProducts, getTopSellers, getLowStockProducts, e getProductDetails. Se for admin ou vendedor, também tem acesso a getTopBillingBranch, getSalesSummaryByBranch, e getTopSellerByPeriod.
                `}]
            }
        };

        const response = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!response.ok) throw new Error(`Erro no proxy da API: ${response.statusText}`);
        
        const result = await response.json();
        const part = result.candidates[0].content.parts[0];
        
        if (part.functionCall) {
            const { name, args } = part.functionCall;
            let toolResult;

            if (name === 'getSalesSummary' || name === 'getTopSellers') {
                toolResult = await tools[name]();
            } else if (name === 'filterProducts') {
                toolResult = await tools.filterProducts(args.category, args.min_price);
            } else if (name === 'getLowStockProducts') {
                toolResult = await tools.getLowStockProducts(args.limit, args.filial);
            } else if (name === 'getTopBillingBranch' || name === 'getTopSellerByPeriod') {
                toolResult = await tools[name](args.period);
            } else if (name === 'getSalesSummaryByBranch') {
                toolResult = await tools.getSalesSummaryByBranch(args.period, args.branch);
            } else if (name === 'getProductDetails') {
                toolResult = await tools.getProductDetails(args.identifier);
            }
            
            chatHistory.push({ role: "model", parts: [{ functionCall: { name, args } }] });
            chatHistory.push({ role: "function", parts: [{ functionResponse: { name, response: { result: toolResult } } }] });
            
            return await getGeminiResponse();
        }
        
        chatHistory.push(result.candidates[0].content);
        return part.text;
    }

    // --- Lógica para o Ollama (com simulação de Tool Calling) ---

    async function getOllamaResponse(prompt) {
        let systemPrompt = `
            Você é um assistente de negócios. Você tem acesso a várias ferramentas.
            Se o utilizador perguntar "quem é você?", apresente-se e descreva as suas capacidades com base nas ferramentas que conhece.
            
            Ferramentas disponíveis para todos:
            1. getSalesSummary()
            2. filterProducts(category: string, min_price: number)
            3. getTopSellers()
            4. getLowStockProducts(limit: number, filial?: string)
            5. getProductDetails(identifier: string)
        `;

        if (userRole === 'admin' || userRole === 'vendedor') {
            systemPrompt += `
            Ferramentas adicionais para si:
            6. getTopBillingBranch(period: string) -> period pode ser 'day', 'week', 'month'.
            7. getSalesSummaryByBranch(period: string, branch: string)
            8. getTopSellerByPeriod(period: string)
            `;
        }

        systemPrompt += `
            Instruções de chamada de função:
            - Para getSalesSummary, responda APENAS com: {"functionCall": "getSalesSummary"}
            - Para filterProducts, responda APENAS com: {"functionCall": "filterProducts", "category": "...", "min_price": ...}
            - Para getTopSellers, responda APENAS com: {"functionCall": "getTopSellers"}
            - Para getLowStockProducts, responda APENAS com: {"functionCall": "getLowStockProducts", "limit": ..., "filial": "..."}
            - Para getProductDetails, responda APENAS com: {"functionCall": "getProductDetails", "identifier": "..."}
            - Para getTopBillingBranch, responda APENAS com: {"functionCall": "getTopBillingBranch", "period": "..."}
            - Para getSalesSummaryByBranch, responda APENAS com: {"functionCall": "getSalesSummaryByBranch", "period": "...", "branch": "..."}
            - Para getTopSellerByPeriod, responda APENAS com: {"functionCall": "getTopSellerByPeriod", "period": "..."}
            Para qualquer outra pergunta, responda normalmente.
        `;

        const payload = {
            model: OLLAMA_MODEL,
            prompt: `${systemPrompt}\n\nUtilizador: ${prompt}`,
            stream: false,
            format: "json"
        };

        const response = await fetch(OLLAMA_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!response.ok) throw new Error(`Erro na API do Ollama: ${response.statusText}`);

        const result = await response.json();
        const aiText = result.response.trim();

        try {
            const parsedResponse = JSON.parse(aiText);
            let dataPrompt = "";
            let toolCalled = false;

            switch (parsedResponse.functionCall) {
                case 'getSalesSummary': {
                    const summaryData = await tools.getSalesSummary();
                    dataPrompt = "Aqui estão os dados do resumo de vendas:\n";
                    summaryData.forEach(item => {
                        dataPrompt += `- ${item.filial_nome}: R$ ${item.total_vendas.toFixed(2)}\n`;
                    });
                    dataPrompt += "\nApresente estes dados ao utilizador de forma amigável.";
                    toolCalled = true;
                    break;
                }
                case 'filterProducts': {
                    const { category, min_price } = parsedResponse;
                    const filteredProducts = await tools.filterProducts(category, min_price);
                    dataPrompt = `Aqui está a lista de produtos da categoria '${category}' com preço superior a R$ ${min_price}:\n`;
                    if (!filteredProducts || filteredProducts.length === 0) {
                        dataPrompt = `Não encontrei produtos da categoria '${category}' com preço superior a R$ ${min_price}.`;
                    } else {
                        filteredProducts.forEach(item => {
                            dataPrompt += `- ${item.Nome}: R$ ${item.PrecoSugerido.toFixed(2)}\n`;
                        });
                    }
                    dataPrompt += "\nApresente esta lista ao utilizador.";
                    toolCalled = true;
                    break;
                }
                case 'getTopSellers': {
                    const topSellers = await tools.getTopSellers();
                    dataPrompt = "Aqui está o ranking dos 3 melhores vendedores do mês:\n";
                    if (!topSellers || topSellers.length === 0) {
                        dataPrompt = "Ainda não há dados de vendas suficientes para gerar um ranking este mês.";
                    } else {
                        topSellers.forEach((seller, index) => {
                            dataPrompt += `${index + 1}. ${seller.vendedor_nome}: R$ ${seller.total_vendas.toFixed(2)}\n`;
                        });
                    }
                    dataPrompt += "\nApresente esta informação ao utilizador.";
                    toolCalled = true;
                    break;
                }
                case 'getLowStockProducts': {
                    const { limit, filial } = parsedResponse;
                    const lowStockProducts = await tools.getLowStockProducts(limit, filial);
                    dataPrompt = `Aqui está a lista dos ${limit || 5} produtos com o stock mais baixo`;
                    if (filial) dataPrompt += ` na filial '${filial}'`;
                    dataPrompt += ':\n';
                    if (!lowStockProducts || lowStockProducts.length === 0) {
                        dataPrompt = "Não encontrei produtos com stock baixo para os filtros selecionados.";
                    } else {
                        lowStockProducts.forEach(item => {
                            dataPrompt += `- ${item.produto_nome} (${item.filial_nome}): ${item.quantidade} unidades\n`;
                        });
                    }
                    dataPrompt += "\nApresente esta informação de forma clara ao utilizador.";
                    toolCalled = true;
                    break;
                }
                case 'getTopBillingBranch': {
                    const { period } = parsedResponse;
                    const result = await tools.getTopBillingBranch(period);
                    if (!result) {
                        dataPrompt = `Não há dados de vendas para o período '${period}'.`;
                    } else {
                        dataPrompt = `A filial com maior faturamento no período '${period}' foi '${result.filial_nome}' com um total de R$ ${result.total_faturado.toFixed(2)}.`;
                    }
                    toolCalled = true;
                    break;
                }
                case 'getSalesSummaryByBranch': {
                    const { period, branch } = parsedResponse;
                    const result = await tools.getSalesSummaryByBranch(period, branch);
                    if (!result) {
                        dataPrompt = `Não encontrei a filial '${branch}'.`;
                    } else {
                        dataPrompt = `Resumo da filial '${result.filial_nome}' no período '${period}':\n- Total de Vendas: R$ ${result.total_vendas.toFixed(2)}\n- Nº de Transações: ${result.numero_transacoes}\n- Ticket Médio: R$ ${result.ticket_medio.toFixed(2)}`;
                    }
                    toolCalled = true;
                    break;
                }
                case 'getTopSellerByPeriod': {
                    const { period } = parsedResponse;
                    const result = await tools.getTopSellerByPeriod(period);
                    if (!result) {
                        dataPrompt = `Não há dados de vendas para encontrar o melhor vendedor no período '${period}'.`;
                    } else {
                        dataPrompt = `O melhor vendedor no período '${period}' foi '${result.vendedor_nome}' com um total de R$ ${result.total_vendas.toFixed(2)} em vendas.`;
                    }
                    toolCalled = true;
                    break;
                }
                case 'getProductDetails': {
                    const { identifier } = parsedResponse;
                    const product = await tools.getProductDetails(identifier);
                    if (product.error) {
                        dataPrompt = `Não encontrei nenhum produto com o identificador '${identifier}'.`;
                    } else {
                        dataPrompt = `Aqui estão os detalhes do produto '${product.Nome}':\n` +
                            `- Categoria: ${product.Categoria}\n` +
                            `- Código de Barras: ${product.CodigoBarras}\n` +
                            `- CNAE: ${product.CodigoCNAE}\n` +
                            `- Preço de Custo: R$ ${product.PrecoCusto.toFixed(2)}\n` +
                            `- Preço de Venda: R$ ${product.PrecoSugerido.toFixed(2)}\n` +
                            `- Margem de Lucro: ${product.PercentualLucro.toFixed(2)}%\n` +
                            `- Estoque Total (todas as filiais): ${product.TotalEstoque} unidades`;
                    }
                    toolCalled = true;
                    break;
                }
            }

            if (toolCalled) {
                return await getOllamaFinalAnswer(dataPrompt);
            }

        } catch (e) {
            return aiText;
        }
        return aiText;
    }

    async function getOllamaFinalAnswer(prompt) {
        const finalPayload = { model: OLLAMA_MODEL, prompt: prompt, stream: false };
        const finalResponse = await fetch(OLLAMA_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(finalPayload)
        });
        const finalResult = await finalResponse.json();
        return finalResult.response;
    }

    addMessage("Olá! Como posso ajudar na análise do seu negócio hoje?", 'ai');
});
