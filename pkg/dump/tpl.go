package dump

const transactionTemplate = `
{{ .Date }} * "{{ .Payee }}" "{{ .Desc }}" {{ range $tag := .Tags }}#{{ $tag }}{{ end }}
    {{ range $k, $v := .Metadata -}}
    {{ $k }}:"{{ $v }}"
    {{ end -}}
    {{ if .Postings }}{{ range $i, $p := .Postings }}{{ if $i }}
    {{ end }}{{ $p }}{{ end }}{{ else }}{{ .ToAccount.ToString }} {{ .Amount }} {{ .Unit }}
    {{ .FromAccount.ToString }}{{ end }}
`

const openAccountTemplate = `
2000-01-01 open Equity:OpenBalance
{{ range $name, $account := . }}{{ if ne $name "Equity:OpenBalance" }}2000-01-01 open {{ $name }}{{ if $account.Booking }} "{{ $account.Booking }}"{{ end }}
{{ end }}{{ if eq $account.Type "Assets"}}
{{ if not $account.NoPad }}{{ if $account.FirstTransactionDate }}{{ $account.FirstTransactionDate }}{{ else }}2022-01-01{{ end }} pad {{ $name }} Equity:OpenBalance
{{ end }}2999-01-01 balance {{ $name }} {{ $account.Balance }} USD
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
