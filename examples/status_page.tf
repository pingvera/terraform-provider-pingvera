# Пример: статус-страница со ссылкой на монитор из monitors.tf и тёмной темой.

resource "pingvera_status_page" "example" {
  slug  = "example"
  title = "Статус example.com"
  theme = "dark"

  monitors = [
    pingvera_monitor.example.id,
  ]

  brand_color = "#4f46e5"
  footer_md   = "Вопросы — support@example.com"
}
