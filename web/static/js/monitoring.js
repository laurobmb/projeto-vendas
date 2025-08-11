document.addEventListener('DOMContentLoaded', () => {
    const dataElement = document.getElementById('dashboard-data');
    if (!dataElement) return;

    const dashboardData = JSON.parse(dataElement.textContent || '{}');

    // --- Lógica para formatar e exibir os KPIs ---
    const kpiRevenue = document.getElementById('kpi-revenue');
    const kpiProfitMargin = document.getElementById('kpi-profit-margin');
    const kpiInventoryTurnover = document.getElementById('kpi-inventory-turnover');
    const kpiTransactions = document.getElementById('kpi-transactions');
    const kpiAvgTicket = document.getElementById('kpi-avg-ticket');
    const kpiStockValue = document.getElementById('kpi-stock-value');

    const formatCurrency = (value) => (value || 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
    const formatPercent = (value) => `${(value || 0).toFixed(2)}%`;
    const formatTurnover = (value) => `${(value || 0).toFixed(2)}x`;
    const formatNumber = (value) => (value || 0).toLocaleString('pt-BR');

    if (kpiRevenue) kpiRevenue.textContent = formatCurrency(dashboardData.TotalRevenue);
    if (kpiTransactions) kpiTransactions.textContent = formatNumber(dashboardData.TotalTransactions);
    if (kpiAvgTicket) kpiAvgTicket.textContent = formatCurrency(dashboardData.AverageTicket);
    if (kpiStockValue) kpiStockValue.textContent = formatCurrency(dashboardData.TotalStockValue);

    // CORREÇÃO: Adicionada uma verificação para garantir que FinancialKPIs existe.
    if (dashboardData.FinancialKPIs) {
        if (kpiProfitMargin) kpiProfitMargin.textContent = formatPercent(dashboardData.FinancialKPIs.gross_profit_margin);
        if (kpiInventoryTurnover) kpiInventoryTurnover.textContent = formatTurnover(dashboardData.FinancialKPIs.inventory_turnover);
    }
    
    // --- Fim da atualização ---

    // Seletor de Período
    const periodSelector = document.getElementById('period-select');
    periodSelector.addEventListener('change', (e) => {
        const selectedPeriod = e.target.value;
        window.location.search = `?period=${selectedPeriod}`;
    });

    // Gráfico 1: Vendas por Filial
    const salesCtx = document.getElementById('salesByBranchChart');
    if (salesCtx && dashboardData.SalesByBranch) {
        const salesData = dashboardData.SalesByBranch;
        const labels = [...new Set(salesData.map(d => d.date))].sort();
        const filiais = [...new Set(salesData.map(d => d.filial_nome))];
        const colors = ['#3B82F6', '#10B981', '#F59E0B', '#8B5CF6', '#EF4444'];
        const datasets = filiais.map((filial, index) => ({
            label: filial,
            data: labels.map(label => {
                const dayData = salesData.find(d => d.date === label && d.filial_nome === filial);
                return dayData ? dayData.total_vendas : 0;
            }),
            backgroundColor: colors[index % colors.length],
        }));
        new Chart(salesCtx, { type: 'bar', data: { labels, datasets }, options: { responsive: true, maintainAspectRatio: false, scales: { x: { stacked: true }, y: { stacked: true, beginAtZero: true } } } });
    }

    // Gráfico 2: Top Vendedores
    const sellersCtx = document.getElementById('topSellersChart');
    if (sellersCtx && dashboardData.TopSellers) {
        const topSellers = dashboardData.TopSellers;
        new Chart(sellersCtx, {
            type: 'bar',
            data: {
                labels: topSellers.map(s => s.vendedor_nome),
                datasets: [{
                    label: 'Total de Vendas (R$)',
                    data: topSellers.map(s => s.total_vendas),
                    backgroundColor: ['#10B981', '#F59E0B', '#8B5CF6'],
                }]
            },
            options: { indexAxis: 'y', responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } } }
        });
    }

    // Gráfico 3: Composição do Estoque
    const stockCtx = document.getElementById('stockCompositionChart');
    if (stockCtx && dashboardData.StockComposition) {
        const stockComp = dashboardData.StockComposition;
        new Chart(stockCtx, {
            type: 'doughnut',
            data: {
                labels: stockComp.map(s => s.category),
                datasets: [{
                    label: 'Valor em Estoque (R$)',
                    data: stockComp.map(s => s.value),
                    backgroundColor: ['#3B82F6', '#10B981', '#F59E0B', '#8B5CF6', '#EF4444', '#6B7280'],
                }]
            },
            options: { responsive: true, maintainAspectRatio: false }
        });
    }
});
