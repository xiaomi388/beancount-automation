package dump

const transactionTemplate = `
{{ .Date }} * "{{ .Payee }}" "{{ .Desc }}" {{ range $tag := .Tags }}#{{ $tag }}{{ end }}
    {{ range $k, $v := .Metadata -}}
    {{ $k }}:"{{ $v }}"
    {{ end -}}
    {{ .ToAccount.ToString }} {{ .Amount }} {{ .Unit }}
    {{ .FromAccount.ToString }}
`

const openAccountTemplate = `
{{ range $a, $_ := . }}2000-01-01 open {{ $a }}
{{ end }}`
