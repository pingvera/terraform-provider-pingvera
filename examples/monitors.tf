# Пример: HTTP-монитор с тегами и порогами.

resource "pingvera_monitor" "example" {
  type   = "http"
  name   = "Основной сайт"
  target = "https://example.com"

  interval_s          = 60
  fail_threshold       = 2
  degraded_latency_ms  = 800

  tags = ["prod", "website"]

  # Сырой JSON доп. настроек монитора (см. описание типа http в дашборде).
  config = jsonencode({
    fail_on_4xx = true
  })
}

# Пример: TCP-монитор без доп. конфигурации.
resource "pingvera_monitor" "db_tcp" {
  type   = "tcp"
  name   = "PostgreSQL"
  target = "db.example.com:5432"
}
