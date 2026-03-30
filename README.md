#生成的文件结构
<pre>
gen/
├── model/
│   └── users.go              # GORM 模型，包含钩子函数
├── repository/
│   ├── users_repository.go   # Repository 接口
│   └── users_repository_impl.go  # GORM 实现
├── service/
│   ├── users_service.go      # Service 接口
│   └── users_service_impl.go # Service 实现
├── dto/
│   └── users_dto.go          # 请求/响应 DTO
└── handler/
    └── users_handler.go      # HTTP Handler
</pre>
<pre>
#GORM 特性支持:

生成的代码支持以下 GORM 特性：
软删除：如果表有 deleted_at 字段，自动支持软删除
自动时间戳：支持 created_at、updated_at 自动管理
钩子函数：生成的模型包含 BeforeCreate、BeforeUpdate 钩子
事务支持：可以在 Service 层使用 GORM 事务
关联查询：可根据需要添加 Preload、Joins 等
批量操作：支持批量创建
</pre>
<pre>
#你可以通过修改 main.go 中的配置来定制生成的代码：
cfg := &Config{
    DBType:  "mysql",                                    // 数据库类型
    DSN:     "root:123456@tcp(127.0.0.1:3306)/mydb?...", // 连接字符串
    Table:   "users",                                    // 表名
    Output:  "./internal",                               // 输出目录
    Module:  "github.com/myproject/server",              // Go module
    Package: "internal",                                 // 包名
}
</pre>
