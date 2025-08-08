// Função para abrir um modal pelo seu ID
function openModal(modalID) {
    const modal = document.getElementById(modalID);
    if (modal) {
        modal.classList.remove('hidden');
        modal.classList.add('flex');
    }
}

// Função para fechar um modal pelo seu ID
function closeModal(modalID) {
    const modal = document.getElementById(modalID);
    if (modal) {
        modal.classList.add('hidden');
        modal.classList.remove('flex');
    }
}

// Função assíncrona para abrir o modal de ajuste de stock
async function openAdjustStockModal(productId, productName) {
    const modal = document.getElementById('adjustStockModal');
    const contentDiv = document.getElementById('adjustStockContent');
    
    document.getElementById('adjustStockProductName').textContent = productName;
    contentDiv.innerHTML = '<p class="text-center">A carregar stock...</p>';
    openModal('adjustStockModal');

    try {
        const response = await fetch(`/admin/api/products/${productId}/stock`);
        if (!response.ok) throw new Error('Falha ao obter dados de stock.');
        
        const stockDetails = await response.json();
        
        contentDiv.innerHTML = ''; // Limpa o conteúdo
        if (!stockDetails || stockDetails.length === 0) {
            contentDiv.innerHTML = '<p class="text-center text-gray-500">Este produto não possui stock em nenhuma filial.</p>';
            return;
        }

        const table = document.createElement('table');
        table.className = 'min-w-full';
        table.innerHTML = `
            <thead class="bg-gray-100">
                <tr>
                    <th class="py-2 px-4 text-left">Filial</th>
                    <th class="py-2 px-4 text-right">Stock Atual</th>
                    <th class="py-2 px-4 text-right">Dar Baixa (Qtd)</th>
                    <th class="py-2 px-4 text-center">Ação</th>
                </tr>
            </thead>
        `;
        const tbody = document.createElement('tbody');
        stockDetails.forEach(stock => {
            const row = document.createElement('tr');
            row.className = 'border-b';
            row.innerHTML = `
                <td class="py-2 px-4">${stock.FilialNome}</td>
                <td class="py-2 px-4 text-right font-mono">${stock.Quantidade}</td>
                <td class="py-2 px-4"><input type="number" min="1" max="${stock.Quantidade}" class="w-24 text-right border rounded p-1" id="adjust-qty-${stock.FilialID}"></td>
                <td class="py-2 px-4 text-center">
                    <button onclick="submitStockAdjustment('${productId}', '${stock.FilialID}')" class="bg-orange-500 text-white px-3 py-1 rounded text-sm hover:bg-orange-600">Guardar</button>
                </td>
            `;
            tbody.appendChild(row);
        });
        table.appendChild(tbody);
        contentDiv.appendChild(table);

    } catch (error) {
        contentDiv.innerHTML = `<p class="text-center text-red-500">${error.message}</p>`;
    }
}

// Função assíncrona para submeter o ajuste de stock
async function submitStockAdjustment(productId, filialId) {
    const qtyInput = document.getElementById(`adjust-qty-${filialId}`);
    const quantity = parseInt(qtyInput.value, 10);

    if (isNaN(quantity) || quantity <= 0) {
        alert('Por favor, insira uma quantidade válida para a baixa.');
        return;
    }

    try {
        const response = await fetch('/admin/api/stock/adjust', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                product_id: productId,
                filial_id: filialId,
                quantity: quantity
            })
        });

        const result = await response.json();

        if (!response.ok) {
            throw new Error(result.error || 'Erro desconhecido.');
        }
        
        alert('Baixa no stock realizada com sucesso!');
        location.reload();

    } catch (error) {
        alert(`Erro ao dar baixa no stock: ${error.message}`);
    }
}

// CORREÇÃO: Função movida para este ficheiro partilhado.
// Alterna os campos no modal de adição de stock
function toggleNewProductFields(value) {
    const existingFields = document.getElementById('existingProductFields');
    const newFields = document.getElementById('newProductFields');
    
    // Verifica se os elementos existem antes de tentar aceder às suas propriedades
    if (!existingFields || !newFields) return;

    const existingSelect = existingFields.querySelector('select');
    const newNameInput = newFields.querySelector('input[name="new_product_name"]');
    const newPriceInput = newFields.querySelector('input[name="new_product_price"]');

    if (value === 'new') {
        existingFields.classList.add('hidden');
        newFields.classList.remove('hidden');
        if (existingSelect) existingSelect.required = false;
        if (newNameInput) newNameInput.required = true;
        if (newPriceInput) newPriceInput.required = true;
    } else {
        existingFields.classList.remove('hidden');
        newFields.classList.add('hidden');
        if (existingSelect) existingSelect.required = true;
        if (newNameInput) newNameInput.required = false;
        if (newPriceInput) newPriceInput.required = false;
    }
}

// Garante que o estado inicial está correto ao carregar a página
document.addEventListener('DOMContentLoaded', () => {
    // Procura pelo seletor de tipo de adição em qualquer página que carregue este script
    const addTypeSelector = document.querySelector('select[name="add_type"]');
    if (addTypeSelector) {
        toggleNewProductFields(addTypeSelector.value);
    }
});

// NOVO: Função para abrir e preencher o modal de edição de utilizador.
function openEditUserModal(userId, name, email, role, filialId) {
    const modal = document.getElementById('editUserModal');
    if (!modal) return;

    // Atualiza a ação do formulário para o ID do utilizador correto
    modal.querySelector('form').action = `/admin/users/edit/${userId}`;

    // Preenche os campos do formulário com os dados atuais do utilizador
    modal.querySelector('input[name="name"]').value = name;
    modal.querySelector('input[name="email"]').value = email;
    modal.querySelector('select[name="role"]').value = role;
    modal.querySelector('select[name="filial_id"]').value = filialId || ""; // Lida com filial nula
    modal.querySelector('input[name="password"]').value = ""; // Limpa o campo da senha

    openModal('editUserModal');
}
