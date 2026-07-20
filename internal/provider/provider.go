// Package provider — Terraform-провайдер Pingvera поверх write-API hub
// (/api/v1/monitors, /api/v1/status-pages). Провайдер — ЧИСТЫЙ клиент REST:
// вся бизнес-логика (квоты, валидация, тарифы) остаётся на сервере, здесь
// только сериализация и маппинг в Terraform-схему.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pingvera/terraform-provider-pingvera/internal/client"
)

// defaultEndpoint — прод-инстанс RU (architecture.md: cloud/on-prem из одной
// кодовой базы, RU-инстанс — дефолт; EN-инстанс задаётся endpoint явно).
const defaultEndpoint = "https://app.pingvera.ru"

// pingveraProvider реализует provider.Provider.
type pingveraProvider struct {
	// version — версия сборки провайдера, пробрасывается из main.go (ldflags
	// при релизе, "dev" при локальной сборке).
	version string
}

// New — конструктор для provider.ServeOpts (main.go).
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &pingveraProvider{version: version}
	}
}

// providerModel — конфигурация блока provider "pingvera" { ... }.
type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *pingveraProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pingvera"
	resp.Version = p.version
}

func (p *pingveraProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Провайдер Pingvera — управление мониторами и статус-страницами через write-API hub (app.pingvera.ru / app.pingvera.com).",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				Description: "Базовый URL hub, напр. https://app.pingvera.ru (RU) или https://app.pingvera.com (EN). " +
					"По умолчанию https://app.pingvera.ru; можно задать через переменную окружения PINGVERA_URL.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "API-токен (Настройки → API-токены в дашборде, формат pv_...). " +
					"Нужны скоупы write:monitors и/или write:status-pages в зависимости от используемых ресурсов. " +
					"Можно задать через переменную окружения PINGVERA_TOKEN — не храните токен в .tf в открытом виде.",
			},
		},
	}
}

func (p *pingveraProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := defaultEndpoint
	if v := os.Getenv("PINGVERA_URL"); v != "" {
		endpoint = v
	}
	if !cfg.Endpoint.IsNull() && cfg.Endpoint.ValueString() != "" {
		endpoint = cfg.Endpoint.ValueString()
	}

	token := os.Getenv("PINGVERA_TOKEN")
	if !cfg.Token.IsNull() && cfg.Token.ValueString() != "" {
		token = cfg.Token.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Не задан API-токен Pingvera",
			"Укажите атрибут token в блоке provider \"pingvera\" либо переменную окружения PINGVERA_TOKEN. "+
				"Токен создаётся в дашборде: Настройки → API-токены (нужен скоуп write:monitors и/или write:status-pages).",
		)
		return
	}

	c := client.New(endpoint, token)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *pingveraProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		NewStatusPageResource,
	}
}

func (p *pingveraProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil // на будущее: pingvera_monitor как data source, сейчас не нужен
}
