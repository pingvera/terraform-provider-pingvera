# Resource: pingvera_monitor

Управляет монитором Pingvera (`http`, `tcp`, `dns`, `tls`, `heartbeat`, `wp`,
`domain`, `links`, `rkn`, `mail_loop`). Отражает
`POST/PATCH/DELETE /api/v1/monitors` на hub.

## Пример

```hcl
resource "pingvera_monitor" "example" {
  type   = "http"
  name   = "Основной сайт"
  target = "https://example.com"

  interval_s          = 60
  fail_threshold       = 2
  degraded_latency_ms  = 800
  tags                 = ["prod", "website"]

  config = jsonencode({
    fail_on_4xx = true
  })
}
```

## Аргументы

| Имя | Тип | Обязателен | Описание |
|---|---|---|---|
| `type` | string | да | `http`\|`tcp`\|`dns`\|`tls`\|`heartbeat`\|`wp`\|`domain`\|`links`\|`rkn`\|`mail_loop`. Изменение пересоздаёт ресурс. |
| `name` | string | да | Человекочитаемое имя. |
| `target` | string | да | Цель проверки. Изменение пересоздаёт ресурс. |
| `interval_s` | number | нет | Интервал проверки, 10..86400 секунд. Если не задан — дефолт сервера по типу/тарифу. |
| `fail_threshold` | number | нет | Число подряд неудач до инцидента, 1..10. По умолчанию 2. |
| `degraded_latency_ms` | number | нет | Порог латентности (мс) для статуса `degraded`. |
| `enabled` | bool | нет | Включён ли монитор. По умолчанию `true`. |
| `tags` | set(string) | нет | Свободные метки. |
| `config` | string (JSON) | нет | Доп. настройки монитора как сырой JSON. Сравнение семантическое. |

## Атрибуты, доступные только для чтения

- `id` — `public_id` монитора.

## Импорт

```sh
terraform import pingvera_monitor.example mon_XXXXXXXXXXXX
```

`mon_XXXXXXXXXXXX` — `public_id` монитора (виден в дашборде/через
`GET /api/v1/monitors`).
