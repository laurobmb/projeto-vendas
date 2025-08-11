document.addEventListener('DOMContentLoaded', () => {
    const salesData = window.salesData || [];
    const ctx = document.getElementById('salesByBranchChart');

    if (!ctx || salesData.length === 0) {
        console.log("Gráfico de vendas não será renderizado: sem dados ou canvas não encontrado.");
        return;
    }

    // Processar os dados para o formato do Chart.js
    const labels = [...new Set(salesData.map(d => d.date))].sort();
    const filiais = [...new Set(salesData.map(d => d.filial_nome))];
    
    const colors = ['#3B82F6', '#10B981', '#F59E0B', '#8B5CF6', '#EF4444'];

    const datasets = filiais.map((filial, index) => {
        return {
            label: filial,
            data: labels.map(label => {
                const dayData = salesData.find(d => d.date === label && d.filial_nome === filial);
                return dayData ? dayData.total_vendas : 0;
            }),
            backgroundColor: colors[index % colors.length],
            borderColor: colors[index % colors.length],
            borderWidth: 1
        };
    });

    new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    stacked: true,
                },
                y: {
                    stacked: true,
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return 'R$ ' + value.toLocaleString('pt-BR');
                        }
                    }
                }
            },
            plugins: {
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.parsed.y !== null) {
                                label += new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(context.parsed.y);
                            }
                            return label;
                        }
                    }
                }
            }
        }
    });
});
