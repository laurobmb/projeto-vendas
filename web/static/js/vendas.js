document.addEventListener('DOMContentLoaded', () => {
    const searchInput = document.getElementById('product-search');
    const searchResults = document.getElementById('search-results');
    const filialSelector = document.getElementById('filial-selector');
    let debounceTimer;
    let cart = [];

    // Adiciona um listener para o seletor de filial, se ele existir
    if (filialSelector && filialSelector.tagName === 'SELECT') {
        filialSelector.addEventListener('change', () => {
            cart = []; // Limpa o carrinho ao mudar de filial
            renderCart();
        });
    }

    searchInput.addEventListener('input', () => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            const query = searchInput.value;
            const selectedFilialId = getSelectedFilialId();
            if (query.length > 2 && selectedFilialId) {
                searchProducts(query);
            } else {
                searchResults.classList.add('hidden');
            }
        }, 300);
    });

    // CORREÇÃO: Nova função auxiliar para obter o ID da filial de forma segura.
    function getSelectedFilialId() {
        // Se o seletor existir e for um <select> (página do admin), usa o seu valor.
        if (filialSelector && filialSelector.tagName === 'SELECT') {
            return filialSelector.value;
        }
        // Caso contrário (página do vendedor/estoquista), obtém o valor do input escondido.
        if (filialSelector && filialSelector.tagName === 'INPUT') {
            return filialSelector.value;
        }
        return null;
    }

    async function searchProducts(query) {
        const selectedFilialId = getSelectedFilialId();
        if (!selectedFilialId) {
            console.error("Nenhuma filial selecionada.");
            return;
        }
        try {
            const response = await fetch(`/api/products/search?q=${query}&filial_id=${selectedFilialId}`);
            if (!response.ok) throw new Error('Erro na busca');
            const products = await response.json();
            displaySearchResults(products);
        } catch (error) {
            console.error('Falha ao buscar produtos:', error);
            searchResults.innerHTML = '<div class="p-3 text-red-500">Erro ao buscar.</div>';
            searchResults.classList.remove('hidden');
        }
    }

    function displaySearchResults(products) {
        searchResults.innerHTML = '';
        if (!products || products.length === 0) {
            searchResults.innerHTML = '<div class="p-3 text-gray-500">Nenhum produto encontrado.</div>';
        } else {
            products.forEach(product => {
                const div = document.createElement('div');
                div.className = 'p-3 hover:bg-gray-100 cursor-pointer border-b';
                div.textContent = `${product.Nome} - R$ ${product.PrecoSugerido.toFixed(2)}`;
                div.onclick = () => addProductToCart(product);
                searchResults.appendChild(div);
            });
        }
        searchResults.classList.remove('hidden');
    }

    function addProductToCart(product) {
        searchInput.value = '';
        searchResults.classList.add('hidden');

        const existingItem = cart.find(item => item.ID === product.ID);
        if (existingItem) {
            existingItem.quantity++;
        } else {
            cart.push({ ...product, quantity: 1 });
        }
        renderCart();
    }
    
    function renderCart() {
        const cartItemsBody = document.getElementById('cart-items');
        
        if (cart.length === 0) {
            cartItemsBody.innerHTML = `
                <tr id="empty-cart-row">
                    <td colspan="5" class="text-center py-10 text-gray-500">Nenhum item adicionado.</td>
                </tr>
            `;
            document.getElementById('total-display').textContent = 'R$ 0,00';
        } else {
            cartItemsBody.innerHTML = ''; 
            let total = 0;
            cart.forEach((item, index) => {
                const subtotal = item.PrecoSugerido * item.quantity;
                total += subtotal;

                const row = document.createElement('tr');
                row.className = 'border-b';
                row.innerHTML = `
                    <td class="py-2 px-3">${item.Nome}</td>
                    <td class="py-2 px-3 text-center">
                        <input type="number" value="${item.quantity}" min="1" onchange="updateQuantity(${index}, this.value)" class="w-20 text-center border rounded p-1">
                    </td>
                    <td class="py-2 px-3 text-right">R$ ${item.PrecoSugerido.toFixed(2)}</td>
                    <td class="py-2 px-3 text-right font-semibold">R$ ${subtotal.toFixed(2)}</td>
                    <td class="py-2 px-3 text-center">
                        <button onclick="removeFromCart(${index})" class="text-red-500 hover:text-red-700 font-bold">X</button>
                    </td>
                `;
                cartItemsBody.appendChild(row);
            });
            document.getElementById('total-display').textContent = `R$ ${total.toFixed(2)}`;
        }
        
        const selectedFilialId = getSelectedFilialId();
        const canSell = cart.length > 0 && selectedFilialId;
        document.getElementById('finalize-sale-btn').disabled = !canSell;
        
        searchInput.disabled = !selectedFilialId;
        if (!selectedFilialId) {
            searchInput.placeholder = "Selecione uma filial para começar...";
        } else {
            searchInput.placeholder = "Digite o nome ou código de barras...";
        }
    }

    window.updateQuantity = (index, newQuantity) => {
        const qty = parseInt(newQuantity, 10);
        if (qty > 0) {
            cart[index].quantity = qty;
        } else {
            cart.splice(index, 1);
        }
        renderCart();
    };

    window.removeFromCart = (index) => {
        cart.splice(index, 1);
        renderCart();
    };

    window.finalizeSale = async () => {
        const selectedFilialId = getSelectedFilialId();
        if (cart.length === 0 || !selectedFilialId) {
            alert("Por favor, adicione itens ao carrinho e selecione uma filial.");
            return;
        }

        try {
            const response = await fetch('/api/sales', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    filial_id: selectedFilialId,
                    items: cart.map(item => ({
                        product_id: item.ID,
                        quantity: item.quantity,
                        unit_price: item.PrecoSugerido
                    }))
                })
            });

            const result = await response.json();
            if (!response.ok) {
                throw new Error(result.error || 'Erro desconhecido ao finalizar a venda.');
            }
            
            alert('Venda finalizada com sucesso!');
            cart = [];
            renderCart();

        } catch (error) {
            alert(`Erro: ${error.message}`);
        }
    };

    document.addEventListener('click', (e) => {
        if (!searchInput.contains(e.target) && !searchResults.contains(e.target)) {
            searchResults.classList.add('hidden');
        }
    });

    renderCart();
});
