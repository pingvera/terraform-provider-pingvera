package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pingvera/terraform-provider-pingvera/internal/client"
)

// statusPageResource — ресурс pingvera_status_page. У API нет PATCH — Update
// шлёт PUT с ПОЛНЫМ набором полей (internal/api/statuspages.go
// handleStatusPageUpdate), поэтому Update всегда пересобирает StatusPageInput
// целиком из плана, а не только изменённые поля.
// hexColorRe — та же валидация, что и на сервере (internal/api/statuspages.go
// var hexColorRe), продублирована здесь: провайдер — отдельный модуль, свой
// код импортировать из основного репозитория нельзя (internal/).
var hexColorRe = regexp.MustCompile(`^$|^#[0-9a-fA-F]{6}$`)

type statusPageResource struct {
	client *client.Client
}

func NewStatusPageResource() resource.Resource { return &statusPageResource{} }

func (r *statusPageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

type statusPageResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Slug         types.String `tfsdk:"slug"`
	Title        types.String `tfsdk:"title"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Monitors     types.Set    `tfsdk:"monitors"`
	Hosts        types.Set    `tfsdk:"hosts"`
	LogoURL      types.String `tfsdk:"logo_url"`
	BrandColor   types.String `tfsdk:"brand_color"`
	HeaderMD     types.String `tfsdk:"header_md"`
	FooterMD     types.String `tfsdk:"footer_md"`
	EmbedDomains types.String `tfsdk:"embed_domains"`
	Timezone     types.String `tfsdk:"timezone"`
	Theme        types.String `tfsdk:"theme"`
	CustomDomain types.String `tfsdk:"custom_domain"`
}

func (r *statusPageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Публичная статус-страница Pingvera (набор мониторов/серверов под slug-ом). " +
			"Соответствует POST/PUT/DELETE /api/v1/status-pages на hub.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "public_id страницы, присваивается сервером.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				Required:    true,
				Description: "Публичный адрес страницы: /status/{slug}. 3..40 символов a-z0-9-, без крайних дефисов.",
			},
			"title": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Заголовок страницы.",
			},
			"enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Включена ли страница. Провайдер не управляет этим полем напрямую (в контракте API нет toggle на create/update без остальных полей) — значение всегда читается с сервера.",
			},
			"monitors": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "public_id мониторов, отображаемых на странице.",
			},
			"hosts": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "public_id серверов, отображаемых на странице.",
			},
			"logo_url": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "URL логотипа (только https://).",
			},
			"brand_color": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Акцентный цвет бренда, формат #rrggbb.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(hexColorRe, "формат #rrggbb"),
				},
			},
			"header_md": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Markdown-текст в шапке страницы (до 4000 символов).",
			},
			"footer_md": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Markdown-текст в подвале страницы (до 4000 символов).",
			},
			"embed_domains": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Домены, которым разрешена встройка (iframe): '*' либо список через пробел.",
			},
			"timezone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IANA-имя часового пояса подписей (напр. Europe/Berlin). Пусто = дефолт инстанса.",
			},
			"theme": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Тема оформления: '' (бумага, дефолт) | light | dark | midnight.",
				Validators: []validator.String{
					stringvalidator.OneOf("", "light", "dark", "midnight"),
				},
			},
			"custom_domain": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Кастомный домен страницы (напр. status.klient.ru). Требует тариф с white-label. Пусто — домен не задан.",
			},
		},
	}
}

func (r *statusPageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Неверный тип ProviderData", fmt.Sprintf("ожидался *client.Client, получено %T", req.ProviderData))
		return
	}
	r.client = c
}

// toInput собирает StatusPageInput из модели плана (общий код Create/Update).
func (m *statusPageResourceModel) toInput(ctx context.Context) (client.StatusPageInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	monitors, d := setToStrings(ctx, m.Monitors)
	diags.Append(d...)
	hosts, d := setToStrings(ctx, m.Hosts)
	diags.Append(d...)

	in := client.StatusPageInput{
		Slug:         m.Slug.ValueString(),
		Title:        m.Title.ValueString(),
		Monitors:     monitors,
		Hosts:        hosts,
		LogoURL:      m.LogoURL.ValueString(),
		BrandColor:   m.BrandColor.ValueString(),
		HeaderMD:     m.HeaderMD.ValueString(),
		FooterMD:     m.FooterMD.ValueString(),
		EmbedDomains: m.EmbedDomains.ValueString(),
		Timezone:     m.Timezone.ValueString(),
		Theme:        m.Theme.ValueString(),
	}
	if !m.CustomDomain.IsNull() && !m.CustomDomain.IsUnknown() {
		v := m.CustomDomain.ValueString()
		in.CustomDomain = &v
	}
	return in, diags
}

func (r *statusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusPageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, diags := plan.toInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	page, err := r.client.CreateStatusPage(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Не удалось создать статус-страницу Pingvera", err.Error())
		return
	}

	newState, diags := statusPageToModel(ctx, page)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *statusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusPageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	page, found, err := r.client.FindStatusPage(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Не удалось прочитать статус-страницу Pingvera", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	newState, diags := statusPageToModel(ctx, page)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *statusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan statusPageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state statusPageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in, diags := plan.toInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// PUT — полная замена; enabled сохраняем как в текущем state (сервер
	// трогает enabled только если прислать non-nil, тут просто не меняем).
	page, err := r.client.UpdateStatusPage(ctx, state.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Не удалось обновить статус-страницу Pingvera", err.Error())
		return
	}

	newState, diags := statusPageToModel(ctx, page)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *statusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusPageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteStatusPage(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Не удалось удалить статус-страницу Pingvera", err.Error())
	}
}

func (r *statusPageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// statusPageToModel маппит ответ API (monitors[]/hosts[] как объекты
// {public_id,name}) в set public_id-ов состояния Terraform.
func statusPageToModel(ctx context.Context, page *client.StatusPage) (statusPageResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	monitors, d := types.SetValueFrom(ctx, types.StringType, page.MonitorIDs())
	diags.Append(d...)
	hosts, d := types.SetValueFrom(ctx, types.StringType, page.HostIDs())
	diags.Append(d...)

	return statusPageResourceModel{
		ID:           types.StringValue(page.PublicID),
		Slug:         types.StringValue(page.Slug),
		Title:        types.StringValue(page.Title),
		Enabled:      types.BoolValue(page.Enabled),
		Monitors:     monitors,
		Hosts:        hosts,
		LogoURL:      types.StringValue(page.LogoURL),
		BrandColor:   types.StringValue(page.BrandColor),
		HeaderMD:     types.StringValue(page.HeaderMD),
		FooterMD:     types.StringValue(page.FooterMD),
		EmbedDomains: types.StringValue(page.EmbedDomains),
		Timezone:     types.StringValue(page.Timezone),
		Theme:        types.StringValue(page.Theme),
		CustomDomain: types.StringValue(page.CustomDomain),
	}, diags
}
