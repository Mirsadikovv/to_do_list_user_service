package config

const (
	ERR_INFORMATION      = "The server has received the request and is continuing the process"
	SUCCESS              = "The request was successful"
	ERR_REDIRECTION      = "You have been redirected and the completion of the request requires further action"
	ERR_BADREQUEST       = "Bad request"
	ERR_INTERNAL_SERVER  = "While the request appears to be valid, the server could not complete the request"
	SUPERADMIN_ROLE      = "superadmin"
	TEACHER_ROLE         = "teacher"
	MANAGER_ROLE         = "manager"
	ADMINISTRATOR_ROLE   = "administrator"
	SUPPORT_TEACHER_ROLE = "support_teacher"
	STUDENT_ROLE         = "student"
	SmtpServer           = "smtp.gmail.com"
	SmtpPort             = "587"
	SmtpUsername         = "mirsadikovmirodil52@gmail.com"
	SmtpPassword         = "jrko xhrd tzst ukxl"
)

var SignedKey = []byte("MGJd@Ro]yKoCc)mVY1^c:upz~4rn9Pt!hYd]>c8dt#+%")
