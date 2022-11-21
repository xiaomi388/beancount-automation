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
2000-01-01 open Equity:OpenBalance
{{ range $a, $_ := . }}2000-01-01 open {{ $a }}
{{ end }}
`

//{{ $account := Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }} -}}

const holdingTemplate = `
2000-01-01 open Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }}

{{ Deref .Holding.InstitutionPriceAsOf.Get }} *
    Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }} {{ .Holding.InstitutionValue }} {{ Deref .Holding.IsoCurrencyCode.Get }}
    Equity:OpenBalance
`
