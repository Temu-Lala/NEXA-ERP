package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Analytics (blueprint reporting/dashboards section). This module turns the raw
// school records into decision-grade KPIs without importing the modules it measures: a
// school.analytics.kpi is a metric DEFINITION whose aggregation target is a source model
// addressed by NAME (a plain string), so enrollment, attendance, exam and workload data
// are aggregated generically through the ORM registry rather than through hard-wired
// joins. Each refresh writes a school.analytics.snapshot — a point-in-time value with its
// numerator/denominator and optional academic-year / campus scope — which projects a
// formatted display value. school.analytics.dashboard groups tiles
// (school.analytics.dashboard.widget) that each point at a KPI with a widget style, and a
// school.analytics.alert.rule raises when a KPI's latest snapshot crosses a threshold.
// The rate / average / sum math, the trend and gauge-band classification, the threshold
// comparison and the numerator "in"-list parsing are all pure and unit-tested; a refresh
// cron recomputes every active KPI and re-evaluates the alert rules. Navigated under the
// single School app.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolCampus       = setupmodels.SchoolCampus
)

// MetricCategory groups a KPI or dashboard to a domain of the school.
type MetricCategory string

const (
	CatAcademic   MetricCategory = "academic"
	CatEnrollment MetricCategory = "enrollment"
	CatAttendance MetricCategory = "attendance"
	CatWorkload   MetricCategory = "workload"
	CatOverall    MetricCategory = "overall"
)

// MetricUnit is how a KPI value should be read.
type MetricUnit string

const (
	UnitCount   MetricUnit = "count"
	UnitPercent MetricUnit = "percent"
	UnitAverage MetricUnit = "average"
	UnitSum     MetricUnit = "sum"
)

// Aggregation is the operation performed over the source model.
type Aggregation string

const (
	AggCount   Aggregation = "count"
	AggAverage Aggregation = "average"
	AggSum     Aggregation = "sum"
	AggRate    Aggregation = "rate"
)

// WidgetType is the visual style of a dashboard tile.
type WidgetType string

const (
	WgNumber WidgetType = "number"
	WgGauge  WidgetType = "gauge"
	WgBar    WidgetType = "bar"
	WgTrend  WidgetType = "trend"
)

// Comparison is the alert threshold operator.
type Comparison string

const (
	CmpLt  Comparison = "lt"
	CmpLte Comparison = "lte"
	CmpGt  Comparison = "gt"
	CmpGte Comparison = "gte"
)

// Severity is the importance of an alert rule.
type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

// SchoolAnalyticsKpi defines a metric and how it is computed from a source model.
type SchoolAnalyticsKpi struct {
	sdk.Model `nexa:"model=school.analytics.kpi"`

	Code        sdk.String                    `nexa:"required,index,size=60,string=Code"`
	Name        sdk.String                    `nexa:"required,size=120,string=Name"`
	Description sdk.Text                      `nexa:"string=Description"`
	Category    sdk.Selection[MetricCategory] `nexa:"string=Category,default=overall,index,selection=academic:Academic,enrollment:Enrollment,attendance:Attendance,workload:Teacher Workload,overall:Overall"`
	Unit        sdk.Selection[MetricUnit]     `nexa:"string=Unit,default=count,index,selection=count:Count,percent:Percent,average:Average,sum:Sum"`
	Aggregation sdk.Selection[Aggregation]    `nexa:"string=Aggregation,default=count,index,selection=count:Count,average:Average,sum:Sum,rate:Rate"`

	SourceModel sdk.String `nexa:"index,size=80,string=Source Model"`
	FilterField sdk.String `nexa:"size=60,string=Filter Field"`
	FilterValue sdk.String `nexa:"size=60,string=Filter Value"`
	ValueField  sdk.String `nexa:"size=60,string=Value Field"`
	NumField    sdk.String `nexa:"size=60,string=Numerator Field"`
	NumValues   sdk.String `nexa:"size=120,string=Numerator Values"`

	Sequence sdk.Integer `nexa:"string=Sequence,default=10"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	LatestValue sdk.Float64 `nexa:"string=Latest Value,compute=compute_kpi_latest_value"`
}

// SchoolAnalyticsSnapshot is a point-in-time computed value for a KPI.
type SchoolAnalyticsSnapshot struct {
	sdk.Model `nexa:"model=school.analytics.snapshot"`

	KpiID          sdk.Many2One[SchoolAnalyticsKpi] `nexa:"required,index,string=KPI"`
	SnapshotDate   sdk.Date                         `nexa:"default=today,index,string=Snapshot Date"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	CampusID       sdk.Many2One[SchoolCampus]       `nexa:"index,string=Campus"`

	NumericValue sdk.Float64 `nexa:"default=0,string=Value"`
	Numerator    sdk.Integer `nexa:"default=0,string=Numerator"`
	Denominator  sdk.Integer `nexa:"default=0,string=Denominator"`

	RefreshedBy    sdk.Integer `nexa:"string=Refreshed By"`
	FormattedValue sdk.String  `nexa:"string=Formatted,compute=compute_snapshot_formatted_value"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// SchoolAnalyticsDashboard groups tiles that surface a set of KPIs.
type SchoolAnalyticsDashboard struct {
	sdk.Model `nexa:"model=school.analytics.dashboard"`

	Code        sdk.String                    `nexa:"index,size=60,string=Code"`
	Name        sdk.String                    `nexa:"required,size=120,string=Name"`
	Category    sdk.Selection[MetricCategory] `nexa:"string=Category,default=overall,index,selection=academic:Academic,enrollment:Enrollment,attendance:Attendance,workload:Teacher Workload,overall:Overall"`
	Sequence    sdk.Integer                   `nexa:"string=Sequence,default=10"`
	Description sdk.Text                      `nexa:"string=Description"`

	Widgets sdk.One2Many[SchoolAnalyticsDashboardWidget] `nexa:"string=Tiles,inverse=dashboard_id"`

	Notes       sdk.Text                  `nexa:"string=Notes"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active      sdk.Boolean               `nexa:"string=Active,default=true"`
	WidgetCount sdk.Integer               `nexa:"string=Tiles,compute=compute_dashboard_widget_count"`
}

// SchoolAnalyticsDashboardWidget is one tile binding a dashboard to a KPI.
type SchoolAnalyticsDashboardWidget struct {
	sdk.Model `nexa:"model=school.analytics.dashboard.widget"`

	DashboardID sdk.Many2One[SchoolAnalyticsDashboard] `nexa:"required,index,string=Dashboard"`
	KpiID       sdk.Many2One[SchoolAnalyticsKpi]       `nexa:"required,index,string=KPI"`
	WidgetType  sdk.Selection[WidgetType]              `nexa:"string=Style,default=number,selection=number:Number,gauge:Gauge,bar:Bar,trend:Trend"`
	Color       sdk.String                             `nexa:"size=20,string=Color"`
	Sequence    sdk.Integer                            `nexa:"string=Sequence,default=10"`
	Span        sdk.Integer                            `nexa:"string=Span,default=4"`

	Notes sdk.Text `nexa:"string=Notes"`
}

// SchoolAnalyticsAlertRule raises when a KPI's latest snapshot crosses a threshold.
type SchoolAnalyticsAlertRule struct {
	sdk.Model `nexa:"model=school.analytics.alert.rule"`

	Name       sdk.String                       `nexa:"required,size=120,string=Name"`
	KpiID      sdk.Many2One[SchoolAnalyticsKpi] `nexa:"required,index,string=KPI"`
	Comparison sdk.Selection[Comparison]        `nexa:"string=When,default=lt,index,selection=lt:Value Below,lte:Value At Or Below,gt:Value Above,gte:Value At Or Above"`
	Threshold  sdk.Float64                      `nexa:"default=0,string=Threshold"`
	Severity   sdk.Selection[Severity]          `nexa:"string=Severity,default=warning,index,selection=info:Info,warning:Warning,critical:Critical"`

	NotifyMessage sdk.Text    `nexa:"string=Notification Message"`
	LastTriggered sdk.Date    `nexa:"string=Last Triggered"`
	TriggerCount  sdk.Integer `nexa:"default=0,string=Triggers"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	MeetsNow sdk.Boolean `nexa:"string=Condition Met,compute=compute_alert_meets_now"`
}

func init() {
	sdk.MustRegister("school_analytics",
		&SchoolAnalyticsKpi{},
		&SchoolAnalyticsSnapshot{},
		&SchoolAnalyticsDashboard{},
		&SchoolAnalyticsDashboardWidget{},
		&SchoolAnalyticsAlertRule{},
	)
}
