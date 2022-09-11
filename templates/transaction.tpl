{% for txn in txns %}
{{ txn.date }} * {{ txn.desc }} {{ txn.tag }}
    {{ txn.to_account }} {{ txn.amount }} {{ txn.unit }}
    {{ txn.from_account }}
{% endfor %}


