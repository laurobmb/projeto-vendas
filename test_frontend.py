#!/usr/bin/env python3

import os
import unittest
import sys
import logging
import time
import shutil
import psycopg2
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait, Select
from selenium.webdriver.support import expected_conditions as EC
from selenium.common.exceptions import TimeoutException, NoSuchElementException

# --- Configurações ---
logging.basicConfig(format='%(asctime)s: %(name)s: %(levelname)s: %(message)s', level=logging.INFO, datefmt='%H:%M:%S')
logger = logging.getLogger('VendasSystemTest')

BASE_URL = os.getenv('APP_URL', 'http://127.0.0.1:8080')
DEFAULT_TIMEOUT = 10
IS_CONTAINER = os.getenv('CONTAINER', 'false').lower() == 'true'
STEP_DELAY = 0.5

# --- Credenciais de Teste ---
ADMIN_EMAIL = "admin@teste.com"
VENDEDOR_EMAIL = "vendedor@teste.com"
ESTOQUISTA_EMAIL = "estoquista@teste.com"
TEST_PASS = "senha123"

# --- Variáveis de Ambiente do Banco de Dados ---
DB_NAME = os.getenv('DB_NAME', 'wallmart_test')
DB_USER = os.getenv('DB_USER', 'me')
DB_PASS = os.getenv('DB_PASS', '1q2w3e')
DB_HOST = os.getenv('DB_HOST', 'localhost')
DB_PORT = os.getenv('DB_PORT', '5432')


class VendasSystemTest(unittest.TestCase):
    
    @classmethod
    def setUpClass(cls):
        """Configura o ambiente uma vez antes de todos os testes."""
        cls.photos_dir = os.path.join(os.path.dirname(__file__), "photos")
        if os.path.exists(cls.photos_dir):
            shutil.rmtree(cls.photos_dir)
        os.makedirs(cls.photos_dir)
        logger.info(f"Diretório de screenshots '{cls.photos_dir}' preparado.")

        options = webdriver.ChromeOptions()
        options.add_argument("--start-maximized")

        if IS_CONTAINER:
            logger.info("A rodar em modo container (headless).")
            options.add_argument('--headless')
            options.add_argument('--no-sandbox')
            options.add_argument('--disable-dev-shm-usage')
            options.add_argument('--window-size=1920,1080')
            
        try:
            cls.browser = webdriver.Chrome(options=options)
            cls.wait = WebDriverWait(cls.browser, DEFAULT_TIMEOUT)
        except Exception as e:
            logger.error(f"Não foi possível iniciar o ChromeDriver: {e}")
            cls.browser = None
            sys.exit(1)
            
        cls.prepare_test_database()

    @classmethod
    def tearDownClass(cls):
        """Encerra o navegador e apaga a base de dados de teste."""
        if cls.browser:
            cls.browser.quit()
        
        cls.cleanup_test_database()
        logger.info("Navegador encerrado e base de dados de teste apagada.")

    @classmethod
    def prepare_test_database(cls):
        conn = None
        try:
            conn = psycopg2.connect(dbname='postgres', user=DB_USER, password=DB_PASS, host=DB_HOST, port=DB_PORT)
            conn.autocommit = True
            cursor = conn.cursor()
            cursor.execute(f"DROP DATABASE IF EXISTS {DB_NAME} WITH (FORCE);")
            cursor.execute(f"CREATE DATABASE {DB_NAME};")
            logger.info(f"Base de dados de teste '{DB_NAME}' criada com sucesso.")
        except psycopg2.Error as e:
            logger.error(f"Erro ao preparar a base de dados: {e}")
            sys.exit(1)
        finally:
            if conn:
                conn.close()

        project_root = os.path.abspath(os.path.dirname(__file__))
        env_vars = f"DB_HOST={DB_HOST} DB_PORT={DB_PORT} DB_USER={DB_USER} DB_PASS={DB_PASS} DB_NAME={DB_NAME}"
        
        logger.info("A executar data_manager para inicializar o esquema...")
        os.system(f'cd {project_root} && {env_vars} go run ./cmd/data_manager/main.go -init')
        
        logger.info("A executar populando_banco para adicionar dados de teste...")
        os.system(f'cd {project_root} && {env_vars} go run ./cmd/populando_banco/main.go')
        
        time.sleep(1)

        filial_id = cls.get_first_filial_id()
        if not filial_id:
            logger.error("Nenhuma filial encontrada para associar aos utilizadores de teste.")
            sys.exit(1)
            
        os.system(f'cd {project_root} && {env_vars} go run ./cmd/create_user/main.go -name="Admin Teste" -email="{ADMIN_EMAIL}" -password="{TEST_PASS}" -role="admin"')
        os.system(f'cd {project_root} && {env_vars} go run ./cmd/create_user/main.go -name="Vendedor Teste" -email="{VENDEDOR_EMAIL}" -password="{TEST_PASS}" -role="vendedor" -filialid="{filial_id}"')
        os.system(f'cd {project_root} && {env_vars} go run ./cmd/create_user/main.go -name="Estoquista Teste" -email="{ESTOQUISTA_EMAIL}" -password="{TEST_PASS}" -role="estoquista" -filialid="{filial_id}"')
        logger.info("Utilizadores de teste criados.")

    @classmethod
    def cleanup_test_database(cls):
        conn = None
        try:
            conn = psycopg2.connect(dbname='postgres', user=DB_USER, password=DB_PASS, host=DB_HOST, port=DB_PORT)
            conn.autocommit = True
            cursor = conn.cursor()
            cursor.execute(f"DROP DATABASE IF EXISTS {DB_NAME} WITH (FORCE);")
            logger.info(f"Base de dados de teste '{DB_NAME}' apagada.")
        except psycopg2.Error as e:
            logger.warning(f"Aviso: Não foi possível apagar a base de dados de teste: {e}")
        finally:
            if conn:
                conn.close()
    
    @classmethod
    def get_first_filial_id(cls):
        conn = None
        try:
            conn = psycopg2.connect(dbname=DB_NAME, user=DB_USER, password=DB_PASS, host=DB_HOST, port=DB_PORT)
            cursor = conn.cursor()
            cursor.execute("SELECT id FROM filiais LIMIT 1;")
            result = cursor.fetchone()
            return str(result[0]) if result else None
        except psycopg2.Error as e:
            logger.error(f"Erro ao obter ID da filial: {e}")
            return None
        finally:
            if conn:
                conn.close()

    def _login(self, email, password):
        """Função auxiliar para fazer login."""
        self.browser.get(f'{BASE_URL}/login')
        self.wait.until(EC.visibility_of_element_located((By.ID, "email"))).send_keys(email)
        self.browser.find_element(By.ID, "password").send_keys(password)
        self._take_screenshot("00_tela_login_preenchida")
        self.browser.find_element(By.CSS_SELECTOR, "button[type='submit']").click()

    def _delay(self):
        """Pausa a execução para facilitar a visualização."""
        time.sleep(STEP_DELAY)

    def _take_screenshot(self, name):
        """Tira um screenshot e guarda com um nome descritivo."""
        timestamp = time.strftime("%Y%m%d-%H%M%S")
        screenshot_path = os.path.join(self.photos_dir, f"{timestamp}_{name}.png")
        try:
            self.browser.save_screenshot(screenshot_path)
            logger.info(f"Screenshot guardado em: {screenshot_path}")
        except Exception as e:
            logger.error(f"Falha ao guardar screenshot: {e}")

    def test_01_admin_full_flow(self):
        """Testa o login do admin e o fluxo de CRUD de utilizadores e sócios."""
        logger.info("--- INICIANDO TESTE 01: FLUXO COMPLETO DO ADMIN ---")
        self._login(ADMIN_EMAIL, TEST_PASS)
        
        self.wait.until(EC.url_contains('/admin/dashboard'))
        self.wait.until(EC.title_contains("Painel do Administrador"))
        self._take_screenshot("01_admin_dashboard")
        logger.info("SUCESSO: Login e redirecionamento para o Painel Administrativo corretos.")
        self._delay()

        # --- Teste de CRUD de Utilizador ---
        timestamp = int(time.time())
        novo_user_email = f"user.teste.{timestamp}@email.com"
        logger.info(f"A adicionar novo utilizador: {novo_user_email}")
        
        self.wait.until(EC.element_to_be_clickable((By.XPATH, "//button[normalize-space()='+ Adicionar Utilizador']"))).click()
        modal_user = self.wait.until(EC.visibility_of_element_located((By.ID, "addUserModal")))
        self._take_screenshot("02_admin_modal_adicionar_user")
        modal_user.find_element(By.NAME, "name").send_keys("Utilizador de Teste CRUD")
        modal_user.find_element(By.NAME, "email").send_keys(novo_user_email)
        modal_user.find_element(By.NAME, "password").send_keys("senha123")
        Select(modal_user.find_element(By.NAME, "role")).select_by_visible_text("Vendedor")
        modal_user.find_element(By.XPATH, ".//button[text()='Guardar']").click()
        
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Utilizador adicionado com sucesso!')]")))
        logger.info("SUCESSO: Mensagem de sucesso ao adicionar utilizador exibida.")

        try:
            pagination_span = self.wait.until(EC.visibility_of_element_located((By.XPATH, "(//div[contains(@class, 'justify-center')]/span[contains(text(), 'Página')])[1]")))
            total_pages = int(pagination_span.text.split(' de ')[1])
            if total_pages > 1:
                logger.info(f"Navegando para a última página ({total_pages}) de utilizadores...")
                current_url = self.browser.current_url.split('?')[0]
                self.browser.get(f"{current_url}?page_users={total_pages}")
        except (TimeoutException, IndexError, ValueError):
            logger.info("Paginação não encontrada ou apenas uma página. A procurar na página atual.")

        linha_user = self.wait.until(EC.visibility_of_element_located((By.XPATH, f"//tr[contains(., '{novo_user_email}')]")))
        self._take_screenshot("03_admin_user_adicionado")
        logger.info("SUCESSO: Novo utilizador encontrado na tabela.")
        self._delay()

        logger.info(f"A remover o utilizador: {novo_user_email}")
        linha_user.find_element(By.XPATH, ".//button[normalize-space()='Remover']").click()
        self.wait.until(EC.alert_is_present()).accept()
        
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Utilizador removido com sucesso!')]")))
        logger.info("SUCESSO: Mensagem de sucesso ao remover utilizador exibida.")

        self.assertTrue(self.wait.until(EC.staleness_of(linha_user)))
        self._take_screenshot("04_admin_user_removido")
        logger.info("SUCESSO: Utilizador removido.")
        self._delay()

        # --- Teste de CRUD de Produto ---
        nome_novo_produto = f"Carro Teste Selenium {timestamp}"
        logger.info(f"A adicionar novo produto: {nome_novo_produto}")
        
        self.wait.until(EC.element_to_be_clickable((By.XPATH, "//button[normalize-space()='+ Adicionar Produto']"))).click()
        modal_produto = self.wait.until(EC.visibility_of_element_located((By.ID, "addProductModal")))
        self._take_screenshot("05_admin_modal_adicionar_produto")
        
        modal_produto.find_element(By.NAME, "name").send_keys(nome_novo_produto)
        modal_produto.find_element(By.NAME, "barcode").send_keys(f"789{timestamp}")
        modal_produto.find_element(By.NAME, "preco_custo").send_keys("50000.00")
        modal_produto.find_element(By.NAME, "percentual_lucro").send_keys("20")
        modal_produto.find_element(By.NAME, "imposto_estadual").send_keys("12")
        modal_produto.find_element(By.NAME, "imposto_federal").send_keys("15")
        Select(modal_produto.find_element(By.NAME, "filial_id")).select_by_index(1) # Seleciona a primeira filial da lista
        modal_produto.find_element(By.NAME, "quantity").send_keys("5")
        
        modal_produto.find_element(By.XPATH, ".//button[text()='Guardar']").click()

        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Produto adicionado com sucesso!')]")))
        logger.info("SUCESSO: Mensagem de sucesso ao adicionar produto exibida.")

        # Procura pelo novo produto para validar
        self.browser.find_element(By.NAME, "search_products").send_keys(nome_novo_produto)
        self.browser.find_element(By.XPATH, "//form[contains(@action, '/admin/dashboard')]//button[@type='submit']").click()
        
        self.wait.until(EC.visibility_of_element_located((By.XPATH, f"//td[text()='{nome_novo_produto}']")))
        self._take_screenshot("06_admin_produto_adicionado")
        logger.info("SUCESSO: Novo produto encontrado na tabela após a busca.")
        self._delay()

    def test_02_vendedor_login_and_sale(self):
        """Testa o login do vendedor e simula uma venda completa."""
        logger.info("--- INICIANDO TESTE 02: FLUXO DO VENDEDOR ---")
        self._login(VENDEDOR_EMAIL, TEST_PASS)

        self.wait.until(EC.url_contains('/vendas/terminal'))
        self.wait.until(EC.title_contains("Terminal de Vendas"))
        self._take_screenshot("09_vendedor_terminal_vazio")
        logger.info("SUCESSO: Redirecionado corretamente.")
        
        search_box = self.wait.until(EC.visibility_of_element_located((By.ID, "product-search")))
        
        logger.info("A procurar por um produto...")
        search_box.send_keys("Arroz")
        
        primeiro_resultado = self.wait.until(EC.element_to_be_clickable((By.CSS_SELECTOR, "#search-results div")))
        nome_produto = primeiro_resultado.text.split(' - ')[0]
        logger.info(f"A adicionar o produto '{nome_produto}' ao carrinho.")
        primeiro_resultado.click()
        self._delay()

        self.wait.until(EC.visibility_of_element_located((By.CSS_SELECTOR, "#cart-items tr")))
        self._take_screenshot("10_vendedor_item_no_carrinho")
        logger.info(f"SUCESSO: Item adicionado.")
        self._delay()
        
        logger.info("A finalizar a venda...")
        self.browser.find_element(By.ID, "finalize-sale-btn").click()
        
        try:
            alert = self.wait.until(EC.alert_is_present())
            alert.accept()
            self._take_screenshot("11_vendedor_venda_finalizada")
            logger.info("SUCESSO: Venda finalizada.")
        except TimeoutException:
            self.fail("O alerta de sucesso da venda não apareceu.")

    def test_03_estoquista_login_and_stock_management(self):
        """Testa o login do estoquista e as funcionalidades de gestão de stock."""
        logger.info("--- INICIANDO TESTE 03: FLUXO DO ESTOQUISTA ---")
        self._login(ESTOQUISTA_EMAIL, TEST_PASS)

        self.wait.until(EC.url_contains('/estoque/dashboard'))
        self.wait.until(EC.title_contains("Painel de Stock"))
        self._take_screenshot("12_estoquista_dashboard")
        logger.info("SUCESSO: Redirecionado corretamente.")
        
        # --- Teste de Adição de Stock a Produto Existente ---
        logger.info("A testar a adição de stock a um produto existente...")
        
        primeira_linha = self.wait.until(EC.visibility_of_element_located((By.XPATH, "//table/tbody/tr[1]")))
        nome_produto_existente = primeira_linha.find_element(By.XPATH, "./td[1]").text
        
        add_stock_button = self.wait.until(EC.element_to_be_clickable((By.XPATH, "//button[normalize-space()='+ Adicionar Stock']")))
        add_stock_button.click()
        
        modal = self.wait.until(EC.visibility_of_element_located((By.ID, "addStockModal")))
        self._take_screenshot("13_estoquista_modal_adicionar_existente")
        Select(modal.find_element(By.NAME, "product_id")).select_by_visible_text(nome_produto_existente)
        modal.find_element(By.NAME, "quantity").send_keys("50")
        modal.find_element(By.XPATH, ".//button[text()='Adicionar']").click()
        
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Stock adicionado com sucesso!')]")))
        self._take_screenshot("14_estoquista_stock_adicionado_sucesso")
        logger.info("SUCESSO: Mensagem de sucesso ao adicionar stock exibida.")
        self._delay()

        # --- Teste de Criação de Novo Produto com Stock Inicial ---
        logger.info("A testar a criação de um novo produto com stock inicial...")
        timestamp = int(time.time())
        nome_novo_produto = f"Produto Teste Selenium {timestamp}"

        add_stock_button = self.wait.until(EC.element_to_be_clickable((By.XPATH, "//button[normalize-space()='+ Adicionar Stock']")))
        add_stock_button.click()
        modal = self.wait.until(EC.visibility_of_element_located((By.ID, "addStockModal")))
        
        Select(modal.find_element(By.NAME, "add_type")).select_by_visible_text("Criar Novo Produto")
        
        self.wait.until(EC.visibility_of_element_located((By.ID, "newProductFields")))
        self._take_screenshot("15_estoquista_modal_criar_novo")
        modal.find_element(By.NAME, "new_product_name").send_keys(nome_novo_produto)
        modal.find_element(By.NAME, "new_product_barcode").send_keys(str(timestamp))
        modal.find_element(By.NAME, "new_product_price").send_keys("19.99")
        modal.find_element(By.NAME, "quantity").send_keys("150")
        modal.find_element(By.XPATH, ".//button[text()='Adicionar']").click()

        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Stock adicionado com sucesso!')]")))
        self._take_screenshot("16_estoquista_novo_produto_criado_sucesso")
        logger.info("SUCESSO: Mensagem de sucesso ao criar novo produto exibida.")

    def test_04_admin_monitoring_dashboard(self):
        """Testa o acesso e a visualização do novo dashboard de monitoramento."""
        logger.info("--- INICIANDO TESTE 04: DASHBOARD DE MONITORAMENTO DO ADMIN ---")
        self._login(ADMIN_EMAIL, TEST_PASS)

        logger.info("A navegar para o Dashboard de Monitoramento...")
        self.wait.until(EC.element_to_be_clickable((By.LINK_TEXT, "Monitoramento"))).click()
        
        self.wait.until(EC.url_contains('/admin/monitoring'))
        self.wait.until(EC.title_contains("Dashboard de Monitoramento"))
        self._take_screenshot("17_admin_dashboard_monitoramento")
        logger.info("SUCESSO: Navegou para a página de monitoramento.")
        self._delay()

        logger.info("A verificar a presença dos KPIs...")
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Faturamento Total')]")))
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Total de Transações')]")))
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Ticket Médio')]")))
        logger.info("SUCESSO: KPIs encontrados.")

        logger.info("A verificar a presença do gráfico de vendas...")
        self.wait.until(EC.visibility_of_element_located((By.ID, "salesByBranchChart")))
        logger.info("SUCESSO: Gráfico de vendas encontrado.")

        logger.info("A verificar a presença da lista de alertas de stock...")
        self.wait.until(EC.visibility_of_element_located((By.XPATH, "//*[contains(text(), 'Alertas de Stock Baixo')]")))
        logger.info("SUCESSO: Lista de alertas encontrada.")
        self._delay()


if __name__ == '__main__':
    unittest.main(verbosity=2, failfast=True)
