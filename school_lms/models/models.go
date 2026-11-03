package models

import (
	basemodels "nexa/addons/base/models"
	"nexa/core/sdk"
	schoolmodels "nexa_addons/school/models"
	academicsmodels "nexa_addons/school_academics/models"
	setupmodels "nexa_addons/school_setup/models"
)

// School Learning Management (blueprint section 6). This delivers the LMS chain: a COURSE
// (a subject taught by a teacher to a class/section in a term) that aggregates learning
// MATERIALS, ASSIGNMENTS (with learner SUBMISSIONS that a teacher grades), a reusable
// QUESTION BANK (multiple-choice with options, true/false and short answer) composed into
// QUIZZES that learners sit as auto-graded ATTEMPTS, and per-learner COURSE PROGRESS. The
// grading arithmetic — percentage, MCQ correctness, weighted attempt score, pass
// evaluation and progress completion — is pure and unit-tested; the ORM layer only
// projects count/aggregate computes and orchestrates the lifecycle actions. Courses,
// assignments and quizzes must be published before learners engage; an attempt is graded
// deterministically from its stored answers when submitted. Navigated under the single
// School app alongside the other school modules.

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

type CourseState string

const (
	CourseDraft     CourseState = "draft"
	CoursePublished CourseState = "published"
	CourseArchived  CourseState = "archived"
)

type MaterialType string

const (
	MatDocument MaterialType = "document"
	MatVideo    MaterialType = "video"
	MatLink     MaterialType = "link"
	MatSlide    MaterialType = "slide"
	MatPdf      MaterialType = "pdf"
)

type AssignState string

const (
	AssignDraft     AssignState = "draft"
	AssignPublished AssignState = "published"
	AssignClosed    AssignState = "closed"
)

type SubmissionState string

const (
	SubSubmitted SubmissionState = "submitted"
	SubGraded    SubmissionState = "graded"
)

type QuestionType string

const (
	QMcq   QuestionType = "multiple_choice"
	QTf    QuestionType = "true_false"
	QShort QuestionType = "short_answer"
)

type Difficulty string

const (
	DiffEasy   Difficulty = "easy"
	DiffMedium Difficulty = "medium"
	DiffHard   Difficulty = "hard"
)

type QuizState string

const (
	QuizDraft     QuizState = "draft"
	QuizPublished QuizState = "published"
	QuizClosed    QuizState = "closed"
)

type AttemptState string

const (
	AttInProgress AttemptState = "in_progress"
	AttSubmitted  AttemptState = "submitted"
	AttGraded     AttemptState = "graded"
)

type ProgressStatus string

const (
	ProgNotStarted ProgressStatus = "not_started"
	ProgInProgress ProgressStatus = "in_progress"
	ProgCompleted  ProgressStatus = "completed"
)

// SchoolLmsCourse is a learning container for materials, assignments and quizzes.
type SchoolLmsCourse struct {
	sdk.Model `nexa:"model=school.lms.course"`

	Title          sdk.String                       `nexa:"required,size=200,string=Course Title"`
	Code           sdk.String                       `nexa:"index,size=40,string=Course Code"`
	SubjectID      sdk.Many2One[SchoolSubject]      `nexa:"index,string=Subject"`
	TeacherID      sdk.Many2One[CoreUser]           `nexa:"index,string=Teacher"`
	AcademicYearID sdk.Many2One[SchoolAcademicYear] `nexa:"index,string=Academic Year"`
	TermID         sdk.Many2One[SchoolTerm]         `nexa:"index,string=Term"`
	ClassID        sdk.Many2One[SchoolClass]        `nexa:"index,string=Class"`
	SectionID      sdk.Many2One[SchoolSection]      `nexa:"index,string=Section"`

	Description sdk.Text                   `nexa:"string=Description"`
	StartDate   sdk.Date                   `nexa:"string=Start Date"`
	EndDate     sdk.Date                   `nexa:"string=End Date"`
	State       sdk.Selection[CourseState] `nexa:"string=Status,default=draft,selection=draft:Draft,published:Published,archived:Archived,index"`

	Materials   sdk.One2Many[SchoolLmsMaterial]   `nexa:"string=Materials,inverse=course_id"`
	Assignments sdk.One2Many[SchoolLmsAssignment] `nexa:"string=Assignments,inverse=course_id"`
	Quizzes     sdk.One2Many[SchoolLmsQuiz]       `nexa:"string=Quizzes,inverse=course_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	MaterialCount   sdk.Integer `nexa:"string=Materials,compute=compute_course_material_count"`
	AssignmentCount sdk.Integer `nexa:"string=Assignments,compute=compute_course_assignment_count"`
	QuizCount       sdk.Integer `nexa:"string=Quizzes,compute=compute_course_quiz_count"`
	EnrolledCount   sdk.Integer `nexa:"string=Learners,compute=compute_course_enrolled_count"`
}

// SchoolLmsMaterial is a learning resource attached to a course.
type SchoolLmsMaterial struct {
	sdk.Model `nexa:"model=school.lms.material"`

	CourseID     sdk.Many2One[SchoolLmsCourse] `nexa:"required,index,string=Course"`
	Title        sdk.String                    `nexa:"required,size=200,string=Title"`
	MaterialType sdk.Selection[MaterialType]   `nexa:"string=Type,default=document,selection=document:Document,video:Video,link:Link,slide:Slides,pdf:PDF,index"`
	Link         sdk.String                    `nexa:"size=500,string=URL"`
	FileRef      sdk.String                    `nexa:"size=200,string=File Reference"`
	Description  sdk.Text                      `nexa:"string=Description"`
	Sequence     sdk.Integer                   `nexa:"string=Order,default=0"`
	Published    sdk.Boolean                   `nexa:"string=Published,default=false"`
	CompanyID    sdk.Many2One[CoreCompany]     `nexa:"string=Company"`
}

// SchoolLmsAssignment is a task learners submit for grading.
type SchoolLmsAssignment struct {
	sdk.Model `nexa:"model=school.lms.assignment"`

	CourseID     sdk.Many2One[SchoolLmsCourse] `nexa:"required,index,string=Course"`
	Title        sdk.String                    `nexa:"required,size=200,string=Title"`
	Instructions sdk.Text                      `nexa:"string=Instructions"`
	DueDate      sdk.Date                      `nexa:"index,string=Due Date"`
	MaxScore     sdk.Float64                   `nexa:"string=Max Score,default=100"`
	Weight       sdk.Float64                   `nexa:"string=Weight,default=1"`
	State        sdk.Selection[AssignState]    `nexa:"string=Status,default=draft,selection=draft:Draft,published:Published,closed:Closed,index"`

	Submissions sdk.One2Many[SchoolLmsSubmission] `nexa:"string=Submissions,inverse=assignment_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	SubmissionCount sdk.Integer `nexa:"string=Submissions,compute=compute_assignment_submission_count"`
	GradedCount     sdk.Integer `nexa:"string=Graded,compute=compute_assignment_graded_count"`
}

// SchoolLmsSubmission is a learner's submission against an assignment.
type SchoolLmsSubmission struct {
	sdk.Model `nexa:"model=school.lms.submission"`

	AssignmentID sdk.Many2One[SchoolLmsAssignment] `nexa:"required,index,string=Assignment"`
	StudentID    sdk.Many2One[SchoolStudent]       `nexa:"required,index,string=Student"`
	SubmittedAt  sdk.Date                          `nexa:"string=Submitted On"`
	Content      sdk.Text                          `nexa:"string=Content"`
	FileRef      sdk.String                        `nexa:"size=200,string=File Reference"`
	Late         sdk.Boolean                       `nexa:"string=Late,default=false"`
	State        sdk.Selection[SubmissionState]    `nexa:"string=Status,default=submitted,selection=submitted:Submitted,graded:Graded,index"`
	Score        sdk.Float64                       `nexa:"string=Score"`
	Feedback     sdk.Text                          `nexa:"string=Feedback"`
	GradedByID   sdk.Many2One[CoreUser]            `nexa:"string=Graded By"`
	CompanyID    sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
}

// SchoolLmsQuestion is a reusable item in the question bank.
type SchoolLmsQuestion struct {
	sdk.Model `nexa:"model=school.lms.question"`

	CourseID      sdk.Many2One[SchoolLmsCourse] `nexa:"index,string=Course"`
	SubjectID     sdk.Many2One[SchoolSubject]   `nexa:"index,string=Subject"`
	QuestionText  sdk.Text                      `nexa:"required,string=Question"`
	QuestionType  sdk.Selection[QuestionType]   `nexa:"string=Type,default=multiple_choice,selection=multiple_choice:Multiple Choice,true_false:True / False,short_answer:Short Answer,index"`
	Difficulty    sdk.Selection[Difficulty]     `nexa:"string=Difficulty,default=medium,selection=easy:Easy,medium:Medium,hard:Hard"`
	CorrectAnswer sdk.String                    `nexa:"size=200,string=Correct Answer"`
	Points        sdk.Float64                   `nexa:"string=Points,default=1"`

	Options sdk.One2Many[SchoolLmsQuestionOption] `nexa:"string=Options,inverse=question_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`
	Active    sdk.Boolean               `nexa:"string=Active,default=true"`

	OptionCount sdk.Integer `nexa:"string=Options,compute=compute_question_option_count"`
}

// SchoolLmsQuestionOption is a selectable answer for a multiple-choice question.
type SchoolLmsQuestionOption struct {
	sdk.Model `nexa:"model=school.lms.question.option"`

	QuestionID sdk.Many2One[SchoolLmsQuestion] `nexa:"required,index,string=Question"`
	OptionText sdk.String                      `nexa:"required,size=200,string=Option"`
	Correct    sdk.Boolean                     `nexa:"string=Correct,default=false"`
	Sequence   sdk.Integer                     `nexa:"string=Order,default=0"`
}

// SchoolLmsQuiz is a timed, auto-graded assessment composed from the question bank.
type SchoolLmsQuiz struct {
	sdk.Model `nexa:"model=school.lms.quiz"`

	CourseID         sdk.Many2One[SchoolLmsCourse] `nexa:"required,index,string=Course"`
	Title            sdk.String                    `nexa:"required,size=200,string=Quiz Title"`
	Description      sdk.Text                      `nexa:"string=Description"`
	TimeLimitMinutes sdk.Integer                   `nexa:"string=Time Limit (min),default=0"`
	MaxAttempts      sdk.Integer                   `nexa:"string=Max Attempts,default=1"`
	PassPercentage   sdk.Float64                   `nexa:"string=Pass %,default=50"`
	State            sdk.Selection[QuizState]      `nexa:"string=Status,default=draft,selection=draft:Draft,published:Published,closed:Closed,index"`

	Lines    sdk.One2Many[SchoolLmsQuizLine]    `nexa:"string=Questions,inverse=quiz_id"`
	Attempts sdk.One2Many[SchoolLmsQuizAttempt] `nexa:"string=Attempts,inverse=quiz_id"`

	Notes     sdk.Text                  `nexa:"string=Notes"`
	CompanyID sdk.Many2One[CoreCompany] `nexa:"string=Company"`

	QuestionCount sdk.Integer `nexa:"string=Questions,compute=compute_quiz_question_count"`
	TotalPoints   sdk.Float64 `nexa:"string=Total Points,compute=compute_quiz_total_points"`
	AttemptCount  sdk.Integer `nexa:"string=Attempts,compute=compute_quiz_attempt_count"`
}

// SchoolLmsQuizLine links a question into a quiz with an optional point override.
type SchoolLmsQuizLine struct {
	sdk.Model `nexa:"model=school.lms.quiz.line"`

	QuizID     sdk.Many2One[SchoolLmsQuiz]     `nexa:"required,index,string=Quiz"`
	QuestionID sdk.Many2One[SchoolLmsQuestion] `nexa:"required,index,string=Question"`
	Points     sdk.Float64                     `nexa:"string=Points,default=0"`
	Sequence   sdk.Integer                     `nexa:"string=Order,default=0"`
}

// SchoolLmsQuizAttempt is a learner's sitting of a quiz; it is graded on submit.
type SchoolLmsQuizAttempt struct {
	sdk.Model `nexa:"model=school.lms.quiz.attempt"`

	QuizID      sdk.Many2One[SchoolLmsQuiz] `nexa:"required,index,string=Quiz"`
	StudentID   sdk.Many2One[SchoolStudent] `nexa:"required,index,string=Student"`
	StartedAt   sdk.Date                    `nexa:"string=Started On"`
	SubmittedAt sdk.Date                    `nexa:"string=Submitted On"`
	State       sdk.Selection[AttemptState] `nexa:"string=Status,default=in_progress,selection=in_progress:In Progress,submitted:Submitted,graded:Graded,index"`
	Score       sdk.Float64                 `nexa:"string=Score"`
	MaxPoints   sdk.Float64                 `nexa:"string=Max Points"`
	Percentage  sdk.Float64                 `nexa:"string=Percentage"`
	Passed      sdk.Boolean                 `nexa:"string=Passed,default=false"`

	Answers   sdk.One2Many[SchoolLmsQuizAnswer] `nexa:"string=Answers,inverse=attempt_id"`
	CompanyID sdk.Many2One[CoreCompany]         `nexa:"string=Company"`
}

// SchoolLmsQuizAnswer is one answered question within an attempt.
type SchoolLmsQuizAnswer struct {
	sdk.Model `nexa:"model=school.lms.quiz.answer"`

	AttemptID        sdk.Many2One[SchoolLmsQuizAttempt]    `nexa:"required,index,string=Attempt"`
	QuestionID       sdk.Many2One[SchoolLmsQuestion]       `nexa:"required,index,string=Question"`
	SelectedOptionID sdk.Many2One[SchoolLmsQuestionOption] `nexa:"index,string=Selected Option"`
	AnswerText       sdk.String                            `nexa:"size=500,string=Answer"`
	Correct          sdk.Boolean                           `nexa:"string=Correct,default=false"`
	PointsScored     sdk.Float64                           `nexa:"string=Points Scored,default=0"`
}

// SchoolLmsProgress tracks a learner's completion of a course.
type SchoolLmsProgress struct {
	sdk.Model `nexa:"model=school.lms.progress"`

	CourseID        sdk.Many2One[SchoolLmsCourse] `nexa:"required,index,string=Course"`
	StudentID       sdk.Many2One[SchoolStudent]   `nexa:"required,index,string=Student"`
	Status          sdk.Selection[ProgressStatus] `nexa:"string=Status,default=not_started,selection=not_started:Not Started,in_progress:In Progress,completed:Completed,index"`
	MaterialsViewed sdk.Integer                   `nexa:"string=Materials Viewed,default=0"`
	LastAccessed    sdk.Date                      `nexa:"string=Last Accessed"`
	Notes           sdk.Text                      `nexa:"string=Notes"`
	CompanyID       sdk.Many2One[CoreCompany]     `nexa:"string=Company"`

	PercentComplete sdk.Float64 `nexa:"string=Complete %,compute=compute_progress_percent"`
}

func init() {
	sdk.MustRegister("school_lms",
		&SchoolLmsCourse{},
		&SchoolLmsMaterial{},
		&SchoolLmsAssignment{},
		&SchoolLmsSubmission{},
		&SchoolLmsQuestion{},
		&SchoolLmsQuestionOption{},
		&SchoolLmsQuiz{},
		&SchoolLmsQuizLine{},
		&SchoolLmsQuizAttempt{},
		&SchoolLmsQuizAnswer{},
		&SchoolLmsProgress{},
	)
}
