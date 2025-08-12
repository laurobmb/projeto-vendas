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
        
        stockDetails.forEach(stock => {
            const form = document.createElement('form');
            form.action = '/admin/api/stock/set';
            form.method = 'POST';
            form.className = 'flex items-center justify-between space-x-4 p-2 border-b';

            form.innerHTML = `
                <input type="hidden" name="product_id" value="${productId}">
                <input type="hidden" name="filial_id" value="${stock.FilialID}">
                <span class="flex-1">${stock.FilialNome}</span>
                <div class="flex items-center">
                    <label class="text-sm mr-2">Qtd:</label>
                    <input type="number" name="quantity" value="${stock.Quantidade}" min="0" class="w-24 text-right border rounded p-1">
                </div>
                <button type="submit" class="bg-green-500 text-white px-3 py-1 rounded text-sm hover:bg-green-600">Guardar</button>
            `;
            contentDiv.appendChild(form);
        });

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

// Alterna os campos no modal de adição de stock
function toggleNewProductFields(value) {
    const existingFields = document.getElementById('existingProductFields');
    const newFields = document.getElementById('newProductFields');
    
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
    const addTypeSelector = document.querySelector('select[name="add_type"]');
    if (addTypeSelector) {
        toggleNewProductFields(addTypeSelector.value);
    }
});

// Função para abrir e preencher o modal de edição de utilizador.
function openEditUserModal(userId, name, email, role, filialId) {
    const modal = document.getElementById('editUserModal');
    if (!modal) return;

    modal.querySelector('form').action = `/admin/users/edit/${userId}`;
    modal.querySelector('input[name="name"]').value = name;
    modal.querySelector('input[name="email"]').value = email;
    modal.querySelector('select[name="role"]').value = role;
    modal.querySelector('select[name="filial_id"]').value = filialId || "";
    modal.querySelector('input[name="password"]').value = "";

    openModal('editUserModal');
}

// CORREÇÃO: Adicionada a função que estava em falta.
// Função para abrir e preencher o modal de edição de produto.
function openEditProductModal(productJson) {
    const modal = document.getElementById('editProductModal');
    if (!modal) return;

    const product = JSON.parse(productJson);

    modal.querySelector('form').action = `/admin/products/edit/${product.ID}`;
    modal.querySelector('input[name="name"]').value = product.Nome;
    modal.querySelector('input[name="barcode"]').value = product.CodigoBarras;
    modal.querySelector('select[name="categoria"]').value = product.Categoria || ''; // CORREÇÃO
    modal.querySelector('input[name="codigo_cnae"]').value = product.CodigoCNAE || ''; // ATUALIZADO    
    modal.querySelector('textarea[name="description"]').value = product.Descricao;
    modal.querySelector('input[name="preco_custo"]').value = product.PrecoCusto;
    modal.querySelector('input[name="percentual_lucro"]').value = product.PercentualLucro;
    modal.querySelector('input[name="imposto_estadual"]').value = product.ImpostoEstadual;
    modal.querySelector('input[name="imposto_federal"]').value = product.ImpostoFederal;

    openModal('editProductModal');
}

function openEditSocioModal(socio) {
    const modal = document.getElementById('editSocioModal');
    if (!modal) return;

    const socioData = JSON.parse(socio);

    modal.querySelector('form').action = `/admin/socios/edit/${socioData.ID}`;
    modal.querySelector('input[name="nome"]').value = socioData.Nome;
    modal.querySelector('input[name="email"]').value = socioData.Email;
    modal.querySelector('input[name="telefone"]').value = socioData.Telefone;
    modal.querySelector('input[name="cpf"]').value = socioData.CPF;
    modal.querySelector('input[name="idade"]').value = socioData.Idade;

    openModal('editSocioModal');
}
