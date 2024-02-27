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
{{ range $name, $account := . }}2000-01-01 open {{ $name }}
{{ if eq $account.Type "Assets"}}
{{ $account.FirstTransactionDate }} pad {{ $name }} Equity:OpenBalance
2999-01-01 balance {{ $name }} {{ $account.Balance }} USD
{{ end }}
{{ end }}
`

//{{ $account := Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }} -}}
//{{ Deref .Holding.InstitutionPriceAsOf.Get }} *

const holdingTemplate = `
2000-01-01 open Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }}

2000-01-01 *
    Assets:{{ .Owner }}:{{ Deref .Holding.IsoCurrencyCode.Get }}:{{ .Institution }}:{{ Replace (Deref .Security.Name.Get) }} {{ .Holding.InstitutionValue }} {{ Deref .Holding.IsoCurrencyCode.Get }}
    Equity:OpenBalance
`
