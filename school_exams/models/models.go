package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	academicsmodels "nexa_addons/school_academics/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Examinations & Assessment (blueprint section 5). This delivers the whole
// assessment chain: a reusable, version-free GRADING SCALE (percentage bands -> letter
// grade + GPA points + a pass mark), EXAMS (assessment plans for a subject/class/section
// in a term), a GRADEBOOK of per-learner scores, an aggregated REPORT CARD per learner
// per term/year with an approval gate, and a cumulative TRANSCRIPT across years. The
// flexible grading engine (percentage -> band -> letter/points, weighted GPA, ranking) is
// pure and unit-tested; the ORM layer only projects computes and orchestrates actions.
// Scores can only be recorded once an exam is conducted and are frozen when it is
// published; report cards must be generated from published exams and approved by a
// manager before they feed a transcript.

type (
	CoreUser           = basemodels.CoreUser
	CoreCompany        = basemodels.CoreCompany
	SchoolStudent      = schoolmodels.SchoolStudent
	SchoolClass        = schoolmodels.SchoolClass
	SchoolSection      = schoolmodels.SchoolSection
	SchoolAcademicYear = schoolmodels.SchoolAcademicYear
	SchoolSubject      = academicsmodels.SchoolSubject
	SchoolTerm         = setupmodels.SchoolTerm
)

type ExamType string

const (
	ExamQuiz       ExamType = "quiz"
	ExamAssignment ExamType = "assignment"
	ExamMidterm    ExamType = "midterm"
	ExamFinal      ExamType = "final"
	ExamProject    ExamType = "project"
)

type ExamState string

const (
	ExamDraft     ExamState = "draft"
	ExamScheduled ExamState = "scheduled"
	ExamConducted ExamState = "conducted"
	ExamPublished ExamState = "published"
)

type ReportState string

const (
	RptDraft     ReportState = "draft"
	RptSubmitted ReportState = "submitted"
	RptApproved  ReportState = "approved"
	RptRejected  ReportState = "rejected"
)

type TranscriptState string

const (
	TrxDraft  TranscriptState = "draft"
	TrxIssued TranscriptState = "issued"
)

// SchoolGradingScale is a named grading scheme with percentage bands and a pass mark.
type SchoolGradingScale struct {
	sdk.Model `nexa:"model=school.grading.scale"`

	Name           sdk.String                       `nexa:"required,size=120,string=Name"`
	Code           sdk.String                       `nexa:"size=20,string=Code"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	IsDefault      sdk.Boolean                      `nexa:"string=Default Scale,default=false"`
	PassPercentage sdk.Float64                      `nexa:"string=Pass %,default=40"`

	Bands sdk.One2Many[SchoolGradingBand] `nexa:"string=Bands,inverse=scale_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	BandCount sdk.Integer `nexa:"string=Bands,compute=compute_scale_band_count"`
}

// SchoolGradingBand maps a lower percentage bound to a letter grade and GPA points.
type SchoolGradingBand struct {
	sdk.Model `nexa:"model=school.grading.band"`

	ScaleID       sdk.Many2One[SchoolGradingScale] `nexa:"required,index,string=Scale"`
	MinPercentage sdk.Float64                      `nexa:"required,index,string=Min %" `
	MaxPercentage sdk.Float64                      `nexa:"string=Max %"`
	Grade         sdk.String                       `nexa:"required,size=10,string=Grade"`
	GradePoints   sdk.Float64                      `nexa:"required,string=Grade Points"`
	Remark        sdk.String                       `nexa:"size=60,string=Remark"`
	Sequence      sdk.Integer                      `nexa:"string=Order,default=0"`
}

// SchoolExam is an assessment plan for a subject within a class/section and term.
type SchoolExam struct {
	sdk.Model `nexa:"model=school.exam"`

	Name      sdk.String                  `nexa:"required,size=120,string=Exam Name"`
	ExamType  sdk.Selection[ExamType]     `nexa:"string=Type,default=midterm,selection=quiz:Quiz,assignment:Assignment,midterm:Midterm,final:Final,project:Project,index"`
	SubjectID sdk.Many2One[SchoolSubject] `nexa:"required,index,string=Subject"`
	ClassID   sdk.Many2One[SchoolClass]   `nexa:"index,string=Class"`
	SectionID sdk.Many2One[SchoolSection] `nexa:"index,string=Section"`

	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	TermID         sdk.Many2One[SchoolTerm]         `nexa:"index,string=Term"`
	ScaleID        sdk.Many2One[SchoolGradingScale] `nexa:"index,string=Grading Scale"`

	ExamDate sdk.Date                 `nexa:"index,string=Exam Date"`
	MaxMarks sdk.Float64              `nexa:"required,string=Max Marks,default=100"`
	Weight   sdk.Float64              `nexa:"string=Weight,default=1"`
	State    sdk.Selection[ExamState] `nexa:"string=Status,default=draft,selection=draft:Draft,scheduled:Scheduled,conducted:Conducted,published:Published,index"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	Scores sdk.One2Many[SchoolExamScore] `nexa:"string=Scores,inverse=exam_id"`

	ScoreCount sdk.Integer `nexa:"string=Scores,compute=compute_exam_score_count"`
	AvgPct     sdk.Float64 `nexa:"string=Avg %,compute=compute_exam_avg_pct"`
}

// SchoolExamScore is one learner's mark on an exam (a gradebook cell).
type SchoolExamScore struct {
	sdk.Model `nexa:"model=school.exam.score"`

	ExamID        sdk.Many2One[SchoolExam]    `nexa:"required,index,string=Exam"`
	StudentID     sdk.Many2One[SchoolStudent] `nexa:"required,index,string=Student"`
	MarksObtained sdk.Float64                 `nexa:"string=Marks Obtained,default=0"`
	Absent        sdk.Boolean                 `nexa:"string=Absent,default=false"`
	Remarks       sdk.Text                    `nexa:"string=Remarks"`

	Percentage  sdk.Float64 `nexa:"string=%,compute=compute_score_percentage"`
	Grade       sdk.String  `nexa:"string=Grade,compute=compute_score_grade"`
	GradePoints sdk.Float64 `nexa:"string=Points,compute=compute_score_points"`
}

// SchoolReportCard is a learner's aggregated result for a term/academic year, gated by
// a draft -> submitted -> approved review before it may feed a transcript.
type SchoolReportCard struct {
	sdk.Model `nexa:"model=school.report.card"`

	StudentID      sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"required,index,string=Academic Year"`
	TermID         sdk.Many2One[SchoolTerm]         `nexa:"index,string=Term"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`
	ScaleID        sdk.Many2One[SchoolGradingScale] `nexa:"index,string=Grading Scale"`

	State        sdk.Selection[ReportState] `nexa:"string=Status,default=draft,selection=draft:Draft,submitted:Submitted,approved:Approved,rejected:Rejected,index"`
	ApprovedByID sdk.Many2One[CoreUser]     `nexa:"string=Approved By"`
	ApproveDate  sdk.Date                   `nexa:"string=Approved Date"`
	Remarks      sdk.Text                   `nexa:"string=Overall Remarks"`

	Lines sdk.One2Many[SchoolReportCardLine] `nexa:"string=Subjects,inverse=report_card_id"`

	TotalMarks    sdk.Float64 `nexa:"string=Total Marks,compute=compute_report_total_marks"`
	ObtainedMarks sdk.Float64 `nexa:"string=Obtained,compute=compute_report_obtained"`
	OverallPct    sdk.Float64 `nexa:"string=Overall %,compute=compute_report_overall_pct"`
	Gpa           sdk.Float64 `nexa:"string=GPA,compute=compute_report_gpa"`
	OverallGrade  sdk.String  `nexa:"string=Grade,compute=compute_report_grade"`
	SubjectCount  sdk.Integer `nexa:"string=Subjects,compute=compute_report_subject_count"`
	ClassRank     sdk.Integer `nexa:"string=Rank,compute=compute_report_rank"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolReportCardLine is one subject's aggregated marks on a report card. Values are
// materialized by the grading engine on generation so aggregates read stored fields.
type SchoolReportCardLine struct {
	sdk.Model `nexa:"model=school.report.card.line"`

	ReportCardID  sdk.Many2One[SchoolReportCard] `nexa:"required,index,string=Report Card"`
	SubjectID     sdk.Many2One[SchoolSubject]    `nexa:"required,index,string=Subject"`
	MaxMarks      sdk.Float64                    `nexa:"string=Max Marks,default=0"`
	ObtainedMarks sdk.Float64                    `nexa:"string=Obtained,default=0"`
	Weight        sdk.Float64                    `nexa:"string=Weight,default=1"`
	Percentage    sdk.Float64                    `nexa:"string=%,default=0"`
	Grade         sdk.String                     `nexa:"size=10,string=Grade"`
	GradePoints   sdk.Float64                    `nexa:"string=Points,default=0"`
	Remark        sdk.String                     `nexa:"size=60,string=Remark"`
	Sequence      sdk.Integer                    `nexa:"string=Order,default=0"`
}

// SchoolTranscript is the cumulative academic record of a learner across years/terms.
type SchoolTranscript struct {
	sdk.Model `nexa:"model=school.transcript"`

	StudentID     sdk.Many2One[SchoolStudent]      `nexa:"required,index,string=Student"`
	Title         sdk.String                       `nexa:"size=120,string=Title,default=Academic Transcript"`
	ScaleID       sdk.Many2One[SchoolGradingScale] `nexa:"index,string=Grading Scale"`
	State         sdk.Selection[TranscriptState]   `nexa:"string=Status,default=draft,selection=draft:Draft,issued:Issued,index"`
	IssueDate     sdk.Date                         `nexa:"string=Issue Date"`
	CertificateNo sdk.String                       `nexa:"index,size=60,string=Certificate No"`

	Lines sdk.One2Many[SchoolTranscriptLine] `nexa:"string=Courses,inverse=transcript_id"`

	TotalCredits  sdk.Float64 `nexa:"string=Credits,compute=compute_transcript_credits"`
	CumulativeGpa sdk.Float64 `nexa:"string=CGPA,compute=compute_transcript_gpa"`
	OverallGrade  sdk.String  `nexa:"string=Grade,compute=compute_transcript_grade"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`
}

// SchoolTranscriptLine is one subject's credit-bearing result within a term/year.
type SchoolTranscriptLine struct {
	sdk.Model `nexa:"model=school.transcript.line"`

	TranscriptID   sdk.Many2One[SchoolTranscript]   `nexa:"required,index,string=Transcript"`
	ReportCardID   sdk.Many2One[SchoolReportCard]   `nexa:"index,string=Source Report"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	TermID         sdk.Many2One[SchoolTerm]         `nexa:"index,string=Term"`
	SubjectID      sdk.Many2One[SchoolSubject]      `nexa:"required,index,string=Subject"`
	Credits        sdk.Float64                      `nexa:"string=Credits,default=0"`
	Percentage     sdk.Float64                      `nexa:"string=%,default=0"`
	Grade          sdk.String                       `nexa:"size=10,string=Grade"`
	GradePoints    sdk.Float64                      `nexa:"string=Points,default=0"`
	Sequence       sdk.Integer                      `nexa:"string=Order,default=0"`
}

func init() {
	sdk.MustRegister("school_exams",
		&SchoolGradingScale{},
		&SchoolGradingBand{},
		&SchoolExam{},
		&SchoolExamScore{},
		&SchoolReportCard{},
		&SchoolReportCardLine{},
		&SchoolTranscript{},
		&SchoolTranscriptLine{},
	)
}
