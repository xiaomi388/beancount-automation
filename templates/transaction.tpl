{% for txn in txns %}
{{ txn.date }} * {{ txn.payee }} {{ txn.desc }} {% for tag in txn.tags %}#{{ tag }} {% endfor %}
    {{ txn.to_account }} {{ txn.amount }} {{ txn.unit }}
    {{ txn.from_account }}
{% endfor %}


