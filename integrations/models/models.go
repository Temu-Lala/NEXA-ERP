package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
)

// Integrations (platform connection governance). This app centralises how the ERP talks to
// external systems rather than what it records: a governed CONNECTION describes one outbound
// endpoint (a webhook, a REST API, an SMS or email gateway, a storage service) by its URL,
// HTTP method, authentication mode and a Secret REFERENCE only — never the secret itself,
// which is resolved from the environment at call time. Enabling a connection requires a valid
// HTTPS endpoint; the connection-test action performs a real, SSRF-guarded request and writes
// an append-only DELIVERY row capturing the true status code, latency, request digest, a
// truncated response excerpt and any transport error. Health, days-since-check and
// delivery-count computes project live status from that trail. The URL policy, outcome
// classification, HMAC signing, auth-header construction and latency/health arithmetic are
// pure and unit-tested. Navigated under its own Integrations app.

type (
	CoreUser    = basemodels.CoreUser
	CoreCompany = basemodels.CoreCompany
)

// ConnectionProvider is the kind of external system a connection targets.
type ConnectionProvider string

const (
	ProvWebhook ConnectionProvider = "webhook"
	ProvRest    ConnectionProvider = "rest_api"
	ProvSMS     ConnectionProvider = "sms"
	ProvEmail   ConnectionProvider = "email"
	ProvStorage ConnectionProvider = "storage"
	ProvOther   ConnectionProvider = "other"
)

// AuthMode is how a connection authenticates to its endpoint. The secret is never stored on
// the record; it is resolved from the environment via the Secret Reference at call time.
type AuthMode string

const (
	AuthNone   AuthMode = "none"
	AuthAPIKey AuthMode = "api_key"
	AuthBearer AuthMode = "bearer"
	AuthBasic  AuthMode = "basic"
	AuthHMAC   AuthMode = "hmac"
)

// ConnectionState is the operational status of a connection.
type ConnectionState string

const (
	StDraft  ConnectionState = "draft"
	StActive ConnectionState = "active"
	StPaused ConnectionState = "paused"
	StError  ConnectionState = "error"
)

// DeliveryTrigger is what caused an outbound call to be recorded.
type DeliveryTrigger string

const (
	TrgTest   DeliveryTrigger = "test"
	TrgManual DeliveryTrigger = "manual"
	TrgAuto   DeliveryTrigger = "auto"
)

// HTTPMethod is the verb a connection issues.
type HTTPMethod string

const (
	MGet    HTTPMethod = "get"
	MPost   HTTPMethod = "post"
	MPut    HTTPMethod = "put"
	MPatch  HTTPMethod = "patch"
	MDelete HTTPMethod = "delete"
)

// InboundEventStatus is the lifecycle of a durably stored inbound webhook delivery. An event
// arrives as new (accepted and persisted by the ingress before any processing), is claimed to
// processing by the asynchronous worker, and finishes processed; a delivery refused at the
// gate is recorded rejected, and one the worker could not finish is failed.
type InboundEventStatus string

const (
	EvNew        InboundEventStatus = "new"
	EvProcessing InboundEventStatus = "processing"
	EvProcessed  InboundEventStatus = "processed"
	EvFailed     InboundEventStatus = "failed"
	EvRejected   InboundEventStatus = "rejected"
)

// IntegrationConnection is a governed outbound connector.
type IntegrationConnection struct {
	sdk.Model `nexa:"model=integration.connection"`

	Name     sdk.String                        `nexa:"required,size=140,string=Name"`
	Code     sdk.String                        `nexa:"index,size=60,string=Code"`
	Provider sdk.Selection[ConnectionProvider] `nexa:"string=Type,default=webhook,index,selection=webhook:Webhook,rest_api:REST API,sms:SMS,email:Email,storage:Storage,other:Other"`
	State    sdk.Selection[ConnectionState]    `nexa:"string=Status,default=draft,index,selection=draft:Draft,active:Active,paused:Paused,error:Error"`

	Endpoint        sdk.String                `nexa:"size=500,string=Endpoint URL"`
	HttpMethod      sdk.Selection[HTTPMethod] `nexa:"string=Method,default=post,index,selection=get:GET,post:POST,put:PUT,patch:PATCH,delete:DELETE"`
	AuthMode        sdk.Selection[AuthMode]   `nexa:"string=Authentication,default=none,index,selection=none:None,api_key:API-Key Header,bearer:Bearer Token,basic:Basic (user:pass),hmac:HMAC Signature"`
	HeaderName      sdk.String                `nexa:"size=120,string=Header Name"`
	ApiUser         sdk.String                `nexa:"size=160,string=API User"`
	SecretRef       sdk.String                `nexa:"size=160,string=Secret Reference"`
	TimeoutSeconds  sdk.Integer               `nexa:"string=Timeout (s),default=15"`
	SignPayload     sdk.Boolean               `nexa:"string=Sign Payload"`
	Headers         sdk.Text                  `nexa:"string=Additional Headers (JSON)"`
	PayloadTemplate sdk.Text                  `nexa:"string=Test Payload (JSON)"`

	LastCheck      sdk.DateTime `nexa:"string=Last Check"`
	LastStatusCode sdk.Integer  `nexa:"string=Last Status"`
	LastLatencyMs  sdk.Integer  `nexa:"string=Last Latency (ms)"`
	LastError      sdk.Text     `nexa:"string=Last Error"`

	Deliveries sdk.One2Many[IntegrationDelivery] `nexa:"string=Deliveries,inverse=connection_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	Health         sdk.String  `nexa:"string=Health,compute=computeConnectionHealth"`
	DaysSinceCheck sdk.Integer `nexa:"string=Days Since Check,compute=computeConnectionDaysSinceCheck"`
	DeliveryCount  sdk.Integer `nexa:"string=Deliveries,compute=computeConnectionDeliveryCount"`
}

// IntegrationWebhook is a governed inbound receiver. It owns a secret URL path token (the
// public ingress address carries no other guessable structure) and a Secret REFERENCE — never
// the secret itself — used to verify an HMAC signature over the raw request body. A delivery
// that passes the signature and timestamp-window checks is persisted as an
// integration.inbound.event row BEFORE any processing happens, so nothing an external system
// sent is lost even if a worker crashes; duplicates (same idempotency key on the same
// webhook) are acknowledged without being stored twice. Rotating the token and enabling or
// pausing intake are server-guarded and gated to administrators.
type IntegrationWebhook struct {
	sdk.Model `nexa:"model=integration.webhook"`

	Name  sdk.String                     `nexa:"required,size=140,string=Name"`
	Code  sdk.String                     `nexa:"index,size=60,string=Code"`
	Token sdk.String                     `nexa:"size=64,index,string=Path Token"`
	State sdk.Selection[ConnectionState] `nexa:"string=Status,default=draft,index,selection=draft:Draft,active:Active,paused:Paused,error:Error"`

	ConnectionID sdk.Many2One[IntegrationConnection] `nexa:"string=Linked Connection"`
	Source       sdk.String                          `nexa:"size=40,string=Source System"`

	SecretRef           sdk.String  `nexa:"size=160,string=Secret Reference"`
	SignatureHeader     sdk.String  `nexa:"size=120,string=Signature Header"`
	TimestampHeader     sdk.String  `nexa:"size=120,string=Timestamp Header"`
	ReplayWindowSeconds sdk.Integer `nexa:"string=Replay Window (s),default=300"`
	RequireSignature    sdk.Boolean `nexa:"string=Require Signature,default=true"`

	LastReceived sdk.DateTime              `nexa:"string=Last Delivery"`
	Notes        sdk.Text                  `nexa:"string=Notes"`
	CompanyID    sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active       sdk.Boolean               `nexa:"string=Active,default=true"`

	Events sdk.One2Many[IntegrationInboundEvent] `nexa:"string=Events,inverse=webhook_id"`

	IngressUrl    sdk.String  `nexa:"string=Ingress URL,compute=computeWebhookIngressUrl"`
	ReceivedCount sdk.Integer `nexa:"string=Events,compute=computeWebhookReceivedCount"`
	PendingCount  sdk.Integer `nexa:"string=Pending,compute=computeWebhookPendingCount"`
}

// IntegrationInboundEvent is the durable store of one accepted (or refused) inbound webhook
// delivery. The raw body is never kept whole — only a bounded excerpt and a content hash —
// while the idempotency key, signature verdict and lifecycle status make the queue auditable
// and replay-safe. Processing is performed asynchronously by the scheduled worker, not in the
// request path.
type IntegrationInboundEvent struct {
	sdk.Model `nexa:"model=integration.inbound.event"`

	WebhookID      sdk.Many2One[IntegrationWebhook]  `nexa:"required,index,string=Webhook,ondelete=cascade"`
	EventKey       sdk.String                        `nexa:"size=160,index,string=Idempotency Key"`
	ReceivedAt     sdk.DateTime                      `nexa:"index,string=Received"`
	Status         sdk.Selection[InboundEventStatus] `nexa:"string=Status,default=new,index,selection=new:New,processing:Processing,processed:Processed,failed:Failed,rejected:Rejected"`
	SignatureOk    sdk.Boolean                       `nexa:"string=Signature Verified"`
	Nonce          sdk.String                        `nexa:"size=120,string=Nonce"`
	SourceIp       sdk.String                        `nexa:"size=64,string=Source IP"`
	ContentType    sdk.String                        `nexa:"size=80,string=Content Type"`
	BodyBytes      sdk.Integer                       `nexa:"string=Body Size (bytes)"`
	PayloadExcerpt sdk.Text                          `nexa:"string=Payload Excerpt"`
	AttemptCount   sdk.Integer                       `nexa:"string=Attempts,default=0"`
	ProcessedAt    sdk.DateTime                      `nexa:"string=Processed At"`
	Error          sdk.Text                          `nexa:"string=Error"`
}

// IntegrationDelivery is an append-only record of one outbound call attempt.
type IntegrationDelivery struct {
	sdk.Model `nexa:"model=integration.delivery"`

	ConnectionID    sdk.Many2One[IntegrationConnection] `nexa:"required,index,string=Connection,ondelete=cascade"`
	Timestamp       sdk.DateTime                        `nexa:"index,string=When"`
	Trigger         sdk.Selection[DeliveryTrigger]      `nexa:"string=Trigger,default=test,index,selection=test:Test,manual:Manual,auto:Automatic"`
	Provider        sdk.String                          `nexa:"size=10,index,string=Adapter"`
	Method          sdk.String                          `nexa:"size=10,string=Method"`
	Url             sdk.String                          `nexa:"size=500,string=URL"`
	StatusCode      sdk.Integer                         `nexa:"index,string=Status"`
	Success         sdk.Boolean                         `nexa:"index,string=Success"`
	DurationMs      sdk.Integer                         `nexa:"string=Duration (ms)"`
	RequestDigest   sdk.Text                            `nexa:"string=Request Digest"`
	ResponseExcerpt sdk.Text                            `nexa:"string=Response"`
	Error           sdk.Text                            `nexa:"string=Error"`
	Signature       sdk.String                          `nexa:"size=80,string=Signature"`
	ActorId         sdk.Integer                         `nexa:"string=Actor"`
}

func init() {
	sdk.MustRegister("integrations",
		&IntegrationConnection{},
		&IntegrationDelivery{},
		&IntegrationWebhook{},
		&IntegrationInboundEvent{},
	)
}
