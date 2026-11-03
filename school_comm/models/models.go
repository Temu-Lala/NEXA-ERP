package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	guardiansmodels "nexa_addons/school_guardians/models"
	setupmodels "nexa_addons/school_setup/models"
	staffmodels "nexa_addons/school_staff/models"
)

// School Communication (blueprint section 8). This is the school's messaging and
// notification companion: delivery CHANNELS (email, SMS, in-app, push) with a routing
// priority, targeted ANNOUNCEMENTS (an audience of students/guardians/staff, optionally
// narrowed to a campus, class or section, with a priority and a publish/expiry window)
// that fan out into per-channel NOTIFICATION records which carry a delivery state machine
// (pending, sent, failed, cancelled) with an attempt counter, a maximum-attempt ceiling
// and a scheduled retry date driven by exponential backoff, and a direct MESSAGE desk for
// one-to-one correspondence with a learner or guardian. The retry eligibility and backoff
// arithmetic are pure and unit-tested; a cron dispatches due notifications and a manual
// retry re-queues a failed one. Announcements reuse staff authors (school.staff.member),
// guardians reuse school_guardians and campuses reuse school_setup, so the module reuses
// the existing school data model. Navigated under the single School app.

type (
	CoreUser          = basemodels.CoreUser
	CoreCompany       = basemodels.CoreCompany
	SchoolStudent     = schoolmodels.SchoolStudent
	SchoolClass       = schoolmodels.SchoolClass
	SchoolSection     = schoolmodels.SchoolSection
	SchoolCampus      = setupmodels.SchoolCampus
	SchoolStaffMember = staffmodels.SchoolStaffMember
	SchoolGuardian    = guardiansmodels.SchoolGuardian
)

type ChannelType string

const (
	ChanEmail ChannelType = "email"
	ChanSMS   ChannelType = "sms"
	ChanInApp ChannelType = "in_app"
	ChanPush  ChannelType = "push"
)

type Audience string

const (
	AudAll       Audience = "all"
	AudStudents  Audience = "all_students"
	AudGuardians Audience = "all_guardians"
	AudStaff     Audience = "all_staff"
	AudClass     Audience = "class"
	AudSection   Audience = "section"
)

type Priority string

const (
	PrioNormal Priority = "normal"
	PrioHigh   Priority = "high"
	PrioUrgent Priority = "urgent"
)

type AnnounceState string

const (
	AnnDraft     AnnounceState = "draft"
	AnnPublished AnnounceState = "published"
	AnnArchived  AnnounceState = "archived"
)

type RecipientType string

const (
	RecAll      RecipientType = "all"
	RecStudent  RecipientType = "student"
	RecGuardian RecipientType = "guardian"
	RecStaff    RecipientType = "staff"
)

type NotifyState string

const (
	NotifyPending   NotifyState = "pending"
	NotifySent      NotifyState = "sent"
	NotifyFailed    NotifyState = "failed"
	NotifyCancelled NotifyState = "cancelled"
)

type MsgSenderType string

const (
	SenderStaff MsgSenderType = "staff"
	SenderUser  MsgSenderType = "user"
)

type MsgCategory string

const (
	CatGeneral   MsgCategory = "general"
	CatAcademic  MsgCategory = "academic"
	CatFinance   MsgCategory = "finance"
	CatBehavior  MsgCategory = "behavior"
	CatTransport MsgCategory = "transport"
	CatOther     MsgCategory = "other"
)

type MsgDirection string

const (
	DirInbound  MsgDirection = "inbound"
	DirOutbound MsgDirection = "outbound"
)

type MsgState string

const (
	MsgDraft    MsgState = "draft"
	MsgSent     MsgState = "sent"
	MsgArchived MsgState = "archived"
)

// SchoolCommunicationChannel is a delivery medium with a routing priority.
type SchoolCommunicationChannel struct {
	sdk.Model `nexa:"model=school.communication.channel"`

	Name        sdk.String                 `nexa:"required,size=120,string=Channel Name"`
	Code        sdk.String                 `nexa:"index,size=40,string=Code"`
	ChannelType sdk.Selection[ChannelType] `nexa:"string=Type,default=email,selection=email:Email,sms:SMS,in_app:In-App,push:Push,index"`
	Priority    sdk.Integer                `nexa:"string=Priority,default=100"`
	Description sdk.Text                   `nexa:"string=Description"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolAnnouncement is a targeted broadcast that fans out into notifications.
type SchoolAnnouncement struct {
	sdk.Model `nexa:"model=school.announcement"`

	Title    sdk.String              `nexa:"required,size=200,string=Title"`
	Summary  sdk.String              `nexa:"size=500,string=Summary"`
	Body     sdk.Text                `nexa:"string=Body"`
	Audience sdk.Selection[Audience] `nexa:"string=Audience,default=all,selection=all:Everyone,all_students:All Students,all_guardians:All Guardians,all_staff:All Staff,class:Class,section:Section,index"`

	CampusID  sdk.Many2One[SchoolCampus]  `nexa:"index,string=Campus"`
	ClassID   sdk.Many2One[SchoolClass]   `nexa:"index,string=Class"`
	SectionID sdk.Many2One[SchoolSection] `nexa:"index,string=Section"`

	Priority    sdk.Selection[Priority]         `nexa:"string=Priority,default=normal,selection=normal:Normal,high:High,urgent:Urgent,index"`
	PublishDate sdk.Date                        `nexa:"index,string=Publish Date"`
	ExpiryDate  sdk.Date                        `nexa:"string=Expiry Date"`
	State       sdk.Selection[AnnounceState]    `nexa:"string=Status,default=draft,selection=draft:Draft,published:Published,archived:Archived,index"`
	AuthorID    sdk.Many2One[SchoolStaffMember] `nexa:"index,string=Author"`

	Notifications sdk.One2Many[SchoolNotification] `nexa:"string=Notifications,inverse=announcement_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	RecipientCount sdk.Integer `nexa:"string=Recipients,compute=compute_announce_recipient_count"`
	DeliveredCount sdk.Integer `nexa:"string=Delivered,compute=compute_announce_delivered_count"`
	DeliveryStatus sdk.String  `nexa:"string=Delivery,compute=compute_announce_delivery_status"`
}

// SchoolNotification is a per-channel delivery with a retry-aware state machine.
type SchoolNotification struct {
	sdk.Model `nexa:"model=school.notification"`

	AnnouncementID sdk.Many2One[SchoolAnnouncement]         `nexa:"index,string=Announcement"`
	ChannelID      sdk.Many2One[SchoolCommunicationChannel] `nexa:"required,index,string=Channel"`
	RecipientType  sdk.Selection[RecipientType]             `nexa:"string=Recipient Type,default=all,selection=all:Everyone,student:Student,guardian:Guardian,staff:Staff,index"`
	StudentID      sdk.Many2One[SchoolStudent]              `nexa:"index,string=Student"`
	Subject        sdk.String                               `nexa:"size=200,string=Subject"`
	Body           sdk.Text                                 `nexa:"string=Body"`
	State          sdk.Selection[NotifyState]               `nexa:"string=Status,default=pending,selection=pending:Pending,sent:Sent,failed:Failed,cancelled:Cancelled,index"`
	Attempts       sdk.Integer                              `nexa:"string=Attempts,default=0"`
	MaxAttempts    sdk.Integer                              `nexa:"string=Max Attempts,default=3"`
	ScheduledAt    sdk.Date                                 `nexa:"string=Scheduled"`
	NextRetryAt    sdk.Date                                 `nexa:"index,string=Next Retry"`
	SentAt         sdk.Date                                 `nexa:"string=Sent"`
	LastError      sdk.Text                                 `nexa:"string=Last Error"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

// SchoolMessage is one-to-one correspondence with a learner or guardian.
type SchoolMessage struct {
	sdk.Model `nexa:"model=school.message"`

	Subject       sdk.String                      `nexa:"required,size=200,string=Subject"`
	Body          sdk.Text                        `nexa:"string=Body"`
	Direction     sdk.Selection[MsgDirection]     `nexa:"string=Direction,default=outbound,selection=inbound:Inbound,outbound:Outbound,index"`
	Category      sdk.Selection[MsgCategory]      `nexa:"string=Category,default=general,selection=general:General,academic:Academic,finance:Finance,behavior:Behavior,transport:Transport,other:Other,index"`
	SenderType    sdk.Selection[MsgSenderType]    `nexa:"string=Sender Type,default=staff,selection=staff:Staff,user:User"`
	SenderStaffID sdk.Many2One[SchoolStaffMember] `nexa:"index,string=Sender (Staff)"`
	SenderUserID  sdk.Many2One[CoreUser]          `nexa:"index,string=Sender (User)"`
	StudentID     sdk.Many2One[SchoolStudent]     `nexa:"index,string=Student"`
	GuardianID    sdk.Many2One[SchoolGuardian]    `nexa:"index,string=Guardian"`
	State         sdk.Selection[MsgState]         `nexa:"string=Status,default=draft,selection=draft:Draft,sent:Sent,archived:Archived,index"`
	SentDate      sdk.Date                        `nexa:"string=Sent Date"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
}

func init() {
	sdk.MustRegister("school_comm",
		&SchoolCommunicationChannel{},
		&SchoolAnnouncement{},
		&SchoolNotification{},
		&SchoolMessage{},
	)
}
