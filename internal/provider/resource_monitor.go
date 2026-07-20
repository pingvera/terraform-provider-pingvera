package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pingvera/terraform-provider-pingvera/internal/client"
)

// monitorResource — ресурс pingvera_monitor: 1:1 к сущности "монитор" из
// internal/api/monitors.go. type/target у API неизменяемы через PATCH,
// поэтому в схеме они RequiresReplace.
type monitorResource struct {
	client *client.Client
}

func NewMonitorResource() resource.Resource { return &monitorResource{} }

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

// monitorResourceModel — состояние ресурса в Terraform. Tags — Set (порядок
// значения не имеет, избегаем ложных diff при пересортировке сервером).
type monitorResourceModel struct {
	ID                types.String         `tfsdk:"id"`
	Type              types.String         `tfsdk:"type"`
	Name              types.String         `tfsdk:"name"`
	Target            types.String         `tfsdk:"target"`
	IntervalS         types.Int64          `tfsdk:"interval_s"`
	FailThreshold     types.Int64          `tfsdk:"fail_threshold"`
	DegradedLatencyMs types.Int64          `tfsdk:"degraded_latency_ms"`
	Enabled           types.Bool           `tfsdk:"enabled"`
	Tags              types.Set            `tfsdk:"tags"`
	Config            jsontypes.Normalized `tfsdk:"config"`
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Монитор Pingvera (http/tcp/dns/tls/wp/domain/links/heartbeat). " +
			"Соответствует POST/PATCH/DELETE /api/v1/monitors на hub.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "public_id монитора, присваивается сервером.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Тип монитора: http|tcp|dns|tls|heartbeat|wp|domain|links|rkn|mail_loop.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Человекочитаемое имя монитора.",
			},
			"target": schema.StringAttribute{
				Required:    true,
				Description: "Цель проверки (URL/хост:порт/домен — зависит от type). Неизменяема: смена цели пересоздаёт монитор.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"interval_s": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Интервал проверки, секунды (10..86400). Если не задан — сервер подставляет дефолт по типу/тарифу.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"fail_threshold": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Сколько подряд неудач считать инцидентом (1..10). По умолчанию 2 (сервер).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"degraded_latency_ms": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Порог латентности (мс), выше которого статус degraded.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Включён ли монитор (по умолчанию true).",
			},
			"tags": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Свободные метки монитора (группировка в дашборде).",
			},
			"config": schema.StringAttribute{
				CustomType:  jsontypes.NormalizedType{},
				Optional:    true,
				Description: "Доп. конфигурация монитора как сырой JSON (напр. {\"fail_on_4xx\":true}). Сравнение семантическое — форматирование JSON не даёт лишнего diff.",
			},
		},
	}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.MonitorCreateInput{
		Type:   plan.Type.ValueString(),
		Name:   plan.Name.ValueString(),
		Target: plan.Target.ValueString(),
	}
	if !plan.IntervalS.IsNull() && !plan.IntervalS.IsUnknown() {
		in.IntervalS = int(plan.IntervalS.ValueInt64())
	}
	if !plan.FailThreshold.IsNull() && !plan.FailThreshold.IsUnknown() {
		in.FailThreshold = int(plan.FailThreshold.ValueInt64())
	}
	if !plan.DegradedLatencyMs.IsNull() && !plan.DegradedLatencyMs.IsUnknown() {
		in.DegradedLatencyMs = int(plan.DegradedLatencyMs.ValueInt64())
	}
	tags, diags := setToStrings(ctx, plan.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	in.Tags = tags
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() && plan.Config.ValueString() != "" {
		in.Config = json.RawMessage(plan.Config.ValueString())
	}

	mon, err := r.client.CreateMonitor(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Не удалось создать монитор Pingvera", err.Error())
		return
	}

	// enabled=true при создании всегда (API не принимает enabled на POST),
	// muted нам недоступен через create — приводим модель к ответу сервера.
	newState, diags := monitorToModel(ctx, mon)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mon, found, err := r.client.FindMonitor(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Не удалось прочитать монитор Pingvera", err.Error())
		return
	}
	if !found {
		// Удалён вне Terraform (в дашборде/другим клиентом) — убираем из state,
		// следующий plan предложит пересоздание.
		resp.State.RemoveResource(ctx)
		return
	}

	newState, diags := monitorToModel(ctx, mon)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var in client.MonitorUpdateInput
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		in.Name = &v
	}
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		v := plan.Enabled.ValueBool()
		in.Enabled = &v
	}
	if plan.IntervalS.ValueInt64() != state.IntervalS.ValueInt64() {
		v := int(plan.IntervalS.ValueInt64())
		in.IntervalS = &v
	}
	if plan.FailThreshold.ValueInt64() != state.FailThreshold.ValueInt64() {
		v := int(plan.FailThreshold.ValueInt64())
		in.FailThreshold = &v
	}
	if plan.DegradedLatencyMs.ValueInt64() != state.DegradedLatencyMs.ValueInt64() {
		v := int(plan.DegradedLatencyMs.ValueInt64())
		in.DegradedLatencyMs = &v
	}
	if plan.Config.ValueString() != state.Config.ValueString() {
		raw := json.RawMessage(plan.Config.ValueString())
		if plan.Config.IsNull() || plan.Config.ValueString() == "" {
			raw = json.RawMessage(`{}`)
		}
		in.Config = &raw
	}

	// PATCH только реально изменённых полей (§ready договорённости с API).
	if in != (client.MonitorUpdateInput{}) {
		if err := r.client.UpdateMonitor(ctx, state.ID.ValueString(), in); err != nil {
			resp.Diagnostics.AddError("Не удалось обновить монитор Pingvera", err.Error())
			return
		}
	}

	// Теги API не патчит отдельной ручкой в контракте задачи — но список
	// тегов приходит только через create; чтобы Update не молчал при diff,
	// перечитываем актуальное состояние с сервера после PATCH.
	mon, found, err := r.client.FindMonitor(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Не удалось перечитать монитор Pingvera после обновления", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	newState, diags := monitorToModel(ctx, mon)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMonitor(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Не удалось удалить монитор Pingvera", err.Error())
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// monitorToModel маппит ответ API в модель ресурса Terraform.
func monitorToModel(ctx context.Context, mon *client.Monitor) (monitorResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	tagsSet, d := types.SetValueFrom(ctx, types.StringType, mon.Tags)
	diags.Append(d...)

	cfg := "{}"
	if len(mon.Config) > 0 {
		cfg = string(mon.Config)
	}

	return monitorResourceModel{
		ID:                types.StringValue(mon.PublicID),
		Type:              types.StringValue(mon.Type),
		Name:              types.StringValue(mon.Name),
		Target:            types.StringValue(mon.Target),
		IntervalS:         types.Int64Value(int64(mon.IntervalS)),
		FailThreshold:     types.Int64Value(int64(mon.FailThreshold)),
		DegradedLatencyMs: types.Int64Value(int64(mon.DegradedLatencyMs)),
		Enabled:           types.BoolValue(mon.Enabled),
		Tags:              tagsSet,
		Config:            jsontypes.NewNormalizedValue(cfg),
	}, diags
}

// setToStrings разворачивает types.Set в []string (для полей tags/monitors/hosts).
func setToStrings(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var out []string
	diags := s.ElementsAs(ctx, &out, false)
	return out, diags
}
