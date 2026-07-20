# Resource: pingvera_status_page

Управляет публичной статус-страницей Pingvera (`/status/{slug}`). Отражает
`POST/PUT/DELETE /api/v1/status-pages` на hub. `PUT` — полная замена
конфигурации, поэтому Terraform всегда отправляет весь набор полей на
`update`, а не только изменённые.

## Пример

```hcl
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
```

## Аргументы

| Имя | Тип | Обязателен | Описание |
|---|---|---|---|
| `slug` | string | да | Адрес страницы: `/status/{slug}`. 3..40 символов `a-z0-9-`, без крайних дефисов. |
| `title` | string | нет | Заголовок страницы. |
| `monitors` | set(string) | нет | `public_id` мониторов на странице. |
| `hosts` | set(string) | нет | `public_id` серверов на странице. |
| `logo_url` | string | нет | URL логотипа, только `https://`. |
| `brand_color` | string | нет | Акцентный цвет, формат `#rrggbb`. |
| `header_md` | string | нет | Markdown в шапке (до 4000 символов). |
| `footer_md` | string | нет | Markdown в подвале (до 4000 символов). |
| `embed_domains` | string | нет | Домены встройки (iframe): `*` либо список через пробел. |
| `timezone` | string | нет | IANA-имя часового пояса. Пусто — дефолт инстанса. |
| `theme` | string | нет | `""` (бумага) \| `light` \| `dark` \| `midnight`. |
| `custom_domain` | string | нет | Кастомный домен страницы. Требует тариф с white-label. |

## Атрибуты, доступные только для чтения

- `id` — `public_id` страницы.
- `enabled` — включена ли страница (страницей нельзя управлять напрямую через этот ресурс).

## Импорт

```sh
terraform import pingvera_status_page.example stp_XXXXXXXXXXXX
```

`stp_XXXXXXXXXXXX` — `public_id` страницы (виден в дашборде/через
`GET /api/v1/status-pages`).
