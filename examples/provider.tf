# Конфигурация провайдера Pingvera.
#
# Токен НЕ храните в .tf в открытом виде — задавайте через переменную
# окружения PINGVERA_TOKEN (провайдер читает её автоматически, если атрибут
# token не указан) или через terraform.tfvars, добавленный в .gitignore.

terraform {
  required_providers {
    pingvera = {
      source  = "pingvera/pingvera"
      version = "~> 0.1"
    }
  }
}

provider "pingvera" {
  # endpoint по умолчанию https://app.pingvera.ru (RU-инстанс).
  # Для EN-инстанса: endpoint = "https://app.pingvera.com"
  endpoint = "https://app.pingvera.ru"

  # token можно не указывать здесь — тогда берётся из PINGVERA_TOKEN.
  # token = var.pingvera_token
}
