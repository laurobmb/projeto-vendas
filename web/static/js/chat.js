document.addEventListener('DOMContentLoaded', () => {
    // --- Configurações do Assistente de IA ---
    const OLLAMA_URL = "http://localhost:11434/api/generate";
    const OLLAMA_MODEL = "llama2"; // IMPORTANTE: Mude para o nome do modelo que tem no Ollama (ex: "llama3", "mistral")
    // -----------------------------------------

    const chatIcon = document.getElementById('chat-icon');
    const chatWindow = document.getElementById('chat-window');
    const closeChatBtn = document.getElementById('close-chat');
    const chatForm = document.getElementById('chat-form');
    const chatInput = document.getElementById('chat-input');
    const chatMessages = document.getElementById('chat-messages');

    if (!chatIcon) return; // Não faz nada se o widget não estiver na página

    // Abre e fecha o chat
    chatIcon.addEventListener('click', () => chatWindow.classList.toggle('hidden'));
    closeChatBtn.addEventListener('click', () => chatWindow.classList.add('hidden'));

    // Envio de mensagem
    chatForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const userMessage = chatInput.value.trim();
        if (!userMessage) return;

        addMessage(userMessage, 'user');
        chatInput.value = '';
        showThinkingIndicator();

        try {
            const aiResponse = await getAIResponse(userMessage);
            addMessage(aiResponse, 'ai');
        } catch (error) {
            console.error("Erro ao comunicar com a IA:", error);
            addMessage("Desculpe, ocorreu um erro ao tentar processar o seu pedido. Verifique se o Ollama está a correr.", 'ai-error');
        } finally {
            removeThinkingIndicator();
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

    async function getAIResponse(prompt) {
        const systemPrompt = `
            Você é um assistente de negócios. Você tem acesso a uma ferramenta: getSalesSummary().
            Quando o utilizador pedir um resumo de vendas, faturamento por filial ou algo semelhante,
            responda APENAS com o texto exato: FUNCTION_CALL_GET_SALES_SUMMARY.
            Para qualquer outra pergunta, responda normalmente.
        `;

        const payload = {
            model: OLLAMA_MODEL,
            prompt: `${systemPrompt}\n\nUtilizador: ${prompt}`,
            stream: false
        };

        const response = await fetch(OLLAMA_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            throw new Error(`Erro na API do Ollama: ${response.statusText}`);
        }

        const result = await response.json();
        const aiText = result.response.trim();

        if (aiText === 'FUNCTION_CALL_GET_SALES_SUMMARY') {
            const summaryData = await fetch('/api/sales/summary').then(res => res.json());
            
            let dataPrompt = "Aqui estão os dados do resumo de vendas:\n";
            summaryData.forEach(item => {
                dataPrompt += `- ${item.filial_nome}: R$ ${item.total_vendas.toFixed(2)}\n`;
            });
            dataPrompt += "\nPor favor, apresente estes dados ao utilizador de forma amigável e resumida.";

            const finalPayload = {
                model: OLLAMA_MODEL,
                prompt: dataPrompt,
                stream: false
            };
            const finalResponse = await fetch(OLLAMA_URL, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(finalPayload)
            });
            const finalResult = await finalResponse.json();
            return finalResult.response;
        }

        return aiText;
    }

    addMessage("Olá! Como posso ajudar na análise do seu negócio hoje?", 'ai');
});
