{{define "subject"}}
Welcome to Greenlight!
{{end}}

{{define "plainBody"}}
Hi,

Thanks for signing up for a Greenlight account. We're excited to have you on board!
To activate your account pls use the below activation code on greenlight.com/v1/users/{{.ID}}/activated
Thanks,

Activation Code: {{.Code}}

The Greenlight Team
{{end}}

{{define "htmlBody"}}
<!doctype html>
<html>

<head>
  <meta name="viewport" content="width=device-width" />
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
</head>

<body>
  <p>Hi,</p>
  <p>Thanks for signing up for a Greenlight account. We're excited to have you on board!</p>
  <p>To activate your account pls use the below activation code on greenlight.com/v1/users/{{.ID}}/activated</p>
  <p>Thanks,</p>
  <p>Activation Code: {{.Code}}</p>
  
  <p>The Greenlight Team</p>
</body>
</html>
{{end}}