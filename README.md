# terraform-provider-pingvera

Terraform-провайдер для [Pingvera](https://pingvera.ru) — сервиса
мониторинга «снаружи и изнутри». Провайдер — **чистый клиент** уже
существующего write-API hub (`/api/v1/monitors`, `/api/v1/status-pages`):
никакой бизнес-логики (квоты, тарифы, валидация) здесь нет, всё это остаётся
на сервере.

Это отдельный Go-модуль (`go.mod` со своим module path), изолированный от
основного репозитория Pingvera — зависимости Terraform Plugin Framework не
попадают в основной `go.mod`/`go.sum`. Каталог живёт внутри монорепо
временно; перед публикацией будет вынесен в собственный репозиторий (см.
раздел «Публикация в Terraform Registry» ниже).

## Ресурсы

- `pingvera_monitor` — монитор (http/tcp/dns/tls/wp/domain/links/heartbeat/…). См. [docs/resources/pingvera_monitor.md](docs/resources/pingvera_monitor.md).
- `pingvera_status_page` — публичная статус-страница. См. [docs/resources/pingvera_status_page.md](docs/resources/pingvera_status_page.md).

Примеры конфигурации — в [examples/](examples/).

## Конфигурация провайдера

```hcl
provider "pingvera" {
  endpoint = "https://app.pingvera.ru" # или https://app.pingvera.com (EN-инстанс)
  token    = var.pingvera_token         # лучше через переменную окружения, см. ниже
}
```

| Атрибут | Env fallback | Обязателен | Описание |
|---|---|---|---|
| `endpoint` | `PINGVERA_URL` | нет (дефолт `https://app.pingvera.ru`) | Базовый URL hub. |
| `token` | `PINGVERA_TOKEN` | да | API-токен (`pv_...`), создаётся в дашборде: Настройки → API-токены. Нужны скоупы `write:monitors` и/или `write:status-pages`. |

Токен — секрет, не коммитьте его в `.tf`. Рекомендуемый способ:

```sh
export PINGVERA_TOKEN=pv_...
terraform apply
```

## Локальная разработка

Провайдер не опубликован в Terraform Registry. Чтобы Terraform CLI
использовал локально собранный бинарник вместо скачивания из реестра,
настройте `dev_overrides` в `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "pingvera/pingvera" = "/root/pingvera/terraform-provider-pingvera/bin"
  }
  direct {}
}
```

Сборка и разработка:

```sh
cd terraform-provider-pingvera
go build -o bin/terraform-provider-pingvera .
go vet ./...
go test ./...
```

С `dev_overrides` `terraform init` не нужен (провайдер не тянется из
реестра); `terraform plan`/`apply` сразу используют локальный бинарник.

### Acceptance-тесты (TF_ACC)

В модуле сознательно нет acceptance-тестов (`TF_ACC=1`): они требуют живой
hub и установленный Terraform CLI, что не подходит для CI без реального
окружения. Ручная проверка — через `examples/` на dev-стенде.

### Документация ресурсов

Документация в `docs/` — рукописный Markdown (не генерируется). Если позже
понадобится синхронизировать со схемой автоматически, можно подключить
[`terraform-plugin-docs`](https://github.com/hashicorp/terraform-plugin-docs):

```sh
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate
```

(команда не запускалась — добавит зависимость и потянет доп. тулинг, не
нужный, пока провайдер не публикуется).

## Публикация в Terraform Registry

Не выполнялась и не входит в эту задачу. Когда придёт время публиковать:

1. **Отдельный репозиторий.** Terraform Registry требует репозиторий вида
   `terraform-provider-pingvera` на GitHub (имя аккаунта = `namespace`
   провайдера). Перенести содержимое этого каталога в новый репозиторий
   (сохранив историю через `git subtree split` или начать с чистого листа).
2. **GPG-подпись релизов.** Завести GPG-ключ, добавить публичный ключ в
   настройки провайдера на registry.terraform.io (Publish → Provider →
   Signing Keys).
3. **GitHub Actions + goreleaser.** Стандартный шаблон HashiCorp
   (`.github/workflows/release.yml` + `.goreleaser.yml`) собирает бинарники
   под все платформы, подписывает их GPG-ключом и публикует GitHub Release
   с `terraform-provider-pingvera_X.Y.Z_manifest.json`,
   `_SHA256SUMS`, `_SHA256SUMS.sig`.
4. **`terraform-registry-manifest.json`** в корне репозитория — версия
   протокола провайдера (`protocol_versions: ["6.0"]` для plugin-framework).
5. Зарегистрировать провайдер на registry.terraform.io (GitHub OAuth,
   выбрать репозиторий) — реестр сам подхватывает новые GitHub Release с
   тегами `vX.Y.Z`.

До этого момента провайдер распространяется только через `dev_overrides`
(см. выше) или локальную установку в
`~/.terraform.d/plugins/registry.terraform.io/pingvera/pingvera/<version>/<os>_<arch>/`.

## Контракт API

Провайдер бьёт в существующий write-API hub, аутентификация — Bearer
API-токен (организация определяется токеном, не передаётся в теле запроса):

- Мониторы: `POST /api/v1/monitors`, `GET /api/v1/monitors` (единичного GET
  нет — чтение через список + фильтр по `public_id`), `PATCH
  /api/v1/monitors/{id}`, `DELETE /api/v1/monitors/{id}`. `type`/`target`
  неизменяемы (`RequiresReplace` в схеме).
- Статус-страницы: `POST /api/v1/status-pages`, `GET /api/v1/status-pages`
  (ключ ответа `status_pages`), `PUT /api/v1/status-pages/{id}` (полная
  замена конфигурации), `DELETE /api/v1/status-pages/{id}`.

Контракт сверен с исходным кодом основного репозитория
(`internal/api/monitors.go`, `internal/api/statuspages.go`) на момент
написания провайдера — при изменении API-контракта провайдер нужно
обновить вручную (это не generated-код).
