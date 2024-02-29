О проекте

GoodsPriorityManager — это RESTful API для управления товарами, их приоритетами и кэшированием данных. 
Основная цель проекта — предоставить простой и эффективный способ управления товарами в базе данных с поддержкой кэширования и асинхронной записи логов.

Технологии:
```Go```
```PostgreSQL```
```Redis```
```NATS```
```ClickHouse```

REST API

Получение списка товаров
```GET /goods/list``` OR ```GET /goods/list?limit=int&offset=int```

Добавление нового товара
```POST /goods/create/<projectId>```
```
{
  "name": "<name>"
}
```

Обновление товара
```PATCH /goods/update/<id>/<projectId>```
```{
  "name": "<name>",
  "description": "" // optional field
}
```

Удаление товара
```DELETE /good/remove/<id>/<projectId>```

Изменение приоритета товара
```PATCH /good/reprioritize/<id>/<projectId>```
```
{
  "newPriority": 2
}

```
