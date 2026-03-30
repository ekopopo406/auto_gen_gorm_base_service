package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
)

// 表结构信息
type TableInfo struct {
	Name          string       // 表名
	NameCamel     string       // 驼峰命名（首字母大写）
	NameCamelLow  string       // 驼峰命名（首字母小写）
	Columns       []ColumnInfo // 列信息
	PrimaryKey    ColumnInfo   // 主键列
	HasDeletedAt  bool         // 是否有 deleted_at 字段（软删除）
	HasCreatedAt  bool         // 是否有 created_at 字段
	HasUpdatedAt  bool         // 是否有 updated_at 字段
}

// 列信息
type ColumnInfo struct {
	Name       string // 列名
	NameCamel  string // 驼峰命名
	Type       string // Go类型
	GormType   string // GORM 类型
	DBType     string // 数据库类型
	IsPrimary  bool   // 是否主键
	IsNullable bool   // 是否可为空
	Comment    string // 注释
	IsAutoInc  bool   // 是否自增
}

// 配置
type Config struct {
	DBType   string // mysql, postgres, sqlite
	DSN      string // 数据源名称
	Table    string // 表名
	Output   string // 输出目录
	Module   string // Go module名称
	Package  string // 包名
}

func main() {
	cfg := &Config{
		DBType:  "mysql",
		DSN:     "root:password@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local",
		Table:   "users",
		Output:  "./gen",
		Module:  "github.com/yourusername/yourproject",
		Package: "gen",
	}

	if err := generateCRUD(cfg); err != nil {
		log.Fatal(err)
	}
}

func generateCRUD(cfg *Config) error {
	// 连接数据库
	db, err := sql.Open(cfg.DBType, cfg.DSN)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 获取表结构
	tableInfo, err := getTableInfo(db, cfg.Table)
	if err != nil {
		return fmt.Errorf("获取表结构失败: %v", err)
	}

	// 创建输出目录
	if err := os.MkdirAll(cfg.Output, 0755); err != nil {
		return err
	}

	// 生成 model（GORM 模型）
	if err := generateModel(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 repository 接口
	if err := generateRepositoryInterface(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 repository 实现（GORM）
	if err := generateRepositoryImpl(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 service 接口
	if err := generateServiceInterface(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 service 实现
	if err := generateServiceImpl(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 request/response DTO
	if err := generateDTO(cfg, tableInfo); err != nil {
		return err
	}

	// 生成 handler（HTTP 接口）
	if err := generateHandler(cfg, tableInfo); err != nil {
		return err
	}

	fmt.Printf("✅ CRUD代码生成成功！输出目录: %s\n", cfg.Output)
	return nil
}

func getTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	// 查询列信息
	rows, err := db.Query(`
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_KEY,
			COLUMN_COMMENT,
			EXTRA
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = ? 
		AND TABLE_SCHEMA = DATABASE()
		ORDER BY ORDINAL_POSITION
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	info := &TableInfo{
		Name:         tableName,
		NameCamel:    toCamelCase(tableName, true),
		NameCamelLow: toCamelCase(tableName, false),
		Columns:      []ColumnInfo{},
	}

	var primaryKey *ColumnInfo

	for rows.Next() {
		var colName, dataType, isNullable, colKey, comment, extra string
		if err := rows.Scan(&colName, &dataType, &isNullable, &colKey, &comment, &extra); err != nil {
			return nil, err
		}

		isAutoInc := strings.Contains(extra, "auto_increment")

		col := ColumnInfo{
			Name:       colName,
			NameCamel:  toCamelCase(colName, true),
			DBType:     dataType,
			IsNullable: isNullable == "YES",
			IsPrimary:  colKey == "PRI",
			Comment:    comment,
			IsAutoInc:  isAutoInc,
			Type:       mapDBTypeToGo(dataType, isNullable == "YES", isAutoInc),
			GormType:   mapDBTypeToGorm(dataType),
		}

		info.Columns = append(info.Columns, col)

		if col.IsPrimary {
			primaryKey = &col
		}

		// 检查特殊字段
		if colName == "deleted_at" {
			info.HasDeletedAt = true
		}
		if colName == "created_at" {
			info.HasCreatedAt = true
		}
		if colName == "updated_at" {
			info.HasUpdatedAt = true
		}
	}

	if primaryKey == nil {
		return nil, fmt.Errorf("表 %s 没有主键", tableName)
	}
	info.PrimaryKey = *primaryKey

	return info, nil
}

func mapDBTypeToGo(dbType string, nullable, isAutoInc bool) string {
	goType := map[string]string{
		"int":       "int",
		"bigint":    "int64",
		"tinyint":   "int8",
		"smallint":  "int16",
		"mediumint": "int32",
		"varchar":   "string",
		"text":      "string",
		"longtext":  "string",
		"char":      "string",
		"datetime":  "time.Time",
		"timestamp": "time.Time",
		"date":      "time.Time",
		"time":      "time.Time",
		"decimal":   "float64",
		"float":     "float32",
		"double":    "float64",
		"bool":      "bool",
		"tinyint(1)": "bool",
		"json":      "datatypes.JSON",
		"jsonb":     "datatypes.JSON",
	}[dbType]

	if goType == "" {
		goType = "string"
	}

	// 对于自增主键，即使可以为空也使用普通类型
	if nullable && !isAutoInc && goType != "string" && goType != "datatypes.JSON" {
		goType = "*" + goType
	}

	return goType
}

func mapDBTypeToGorm(dbType string) string {
	gormType := map[string]string{
		"int":       "int",
		"bigint":    "bigint",
		"tinyint":   "tinyint",
		"varchar":   "varchar(255)",
		"text":      "text",
		"longtext":  "longtext",
		"datetime":  "datetime",
		"timestamp": "timestamp",
		"date":      "date",
		"decimal":   "decimal(10,2)",
		"float":     "float",
		"double":    "double",
		"bool":      "bool",
		"json":      "json",
	}[dbType]

	if gormType == "" {
		gormType = dbType
	}

	return gormType
}

func toCamelCase(s string, upperFirst bool) string {
	// 处理特殊前缀（如 id_ 保持 ID）
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part == "id" && i == 0 && !upperFirst {
			parts[i] = "id"
			continue
		}
		if part == "id" {
			parts[i] = "ID"
			continue
		}
		if i == 0 && !upperFirst {
			if len(part) > 0 {
				parts[i] = strings.ToLower(part[:1]) + part[1:]
			}
			continue
		}
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	result := strings.Join(parts, "")
	if !upperFirst && len(result) > 0 {
		result = strings.ToLower(result[:1]) + result[1:]
	}
	return result
}

// 生成 GORM Model
func generateModel(cfg *Config, table *TableInfo) error {
	tmpl := `package model

import (
	"time"
	"gorm.io/gorm"
	"gorm.io/datatypes"
)

// {{.NameCamel}} {{.Comment}} 表名: {{.Name}}
type {{.NameCamel}} struct {
{{- range .Columns}}
	{{.NameCamel}} {{.Type}} ` + "`gorm:\"column:{{.Name}};type:{{.GormType}};{{if .IsPrimary}}primaryKey;{{if .IsAutoInc}}autoIncrement;{{end}}{{end}}{{if .IsNullable}}nullable;{{end}}\" json:\"{{.Name}}\"`" + ` // {{.Comment}}
{{- end}}
}

func ({{.NameCamelLow}} *{{.NameCamel}}) TableName() string {
	return "{{.Name}}"
}

// BeforeCreate GORM 钩子
func ({{.NameCamelLow}} *{{.NameCamel}}) BeforeCreate(tx *gorm.DB) error {
	// TODO: 添加创建前的逻辑
	return nil
}

// BeforeUpdate GORM 钩子
func ({{.NameCamelLow}} *{{.NameCamel}}) BeforeUpdate(tx *gorm.DB) error {
	// TODO: 添加更新前的逻辑
	return nil
}
`

	return generateFile(cfg.Output+"/model/"+table.Name+".go", tmpl, table)
}

// 生成 repository 接口
func generateRepositoryInterface(cfg *Config, table *TableInfo) error {
	tmpl := `package repository

import (
	"context"
	"{{.Module}}/gen/model"
)

// {{.NameCamel}}Repository 定义数据访问接口
type {{.NameCamel}}Repository interface {
	// Create 创建记录
	Create(ctx context.Context, {{.NameCamelLow}} *model.{{.NameCamel}}) error

	// Update 更新记录
	Update(ctx context.Context, {{.NameCamelLow}} *model.{{.NameCamel}}) error

	// Delete 删除记录（根据主键，支持软删除）
	Delete(ctx context.Context, id {{.PrimaryKey.Type}}) error

	// ForceDelete 永久删除记录
	ForceDelete(ctx context.Context, id {{.PrimaryKey.Type}}) error

	// GetByID 根据主键查询
	GetByID(ctx context.Context, id {{.PrimaryKey.Type}}) (*model.{{.NameCamel}}, error)

	// List 分页查询
	List(ctx context.Context, page, pageSize int) ([]*model.{{.NameCamel}}, int64, error)

	// ListByCondition 条件查询
	ListByCondition(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*model.{{.NameCamel}}, int64, error)

	// Exists 检查记录是否存在
	Exists(ctx context.Context, id {{.PrimaryKey.Type}}) (bool, error)

	// Count 统计总数
	Count(ctx context.Context) (int64, error)

	// BatchCreate 批量创建
	BatchCreate(ctx context.Context, {{.NameCamelLow}}s []*model.{{.NameCamel}}) error
}
`

	return generateFile(cfg.Output+"/repository/"+table.Name+"_repository.go", tmpl, table)
}

// 生成 repository 实现（GORM）
func generateRepositoryImpl(cfg *Config, table *TableInfo) error {
	tmpl := `package repository

import (
	"context"
	"fmt"

	"{{.Module}}/gen/model"
	"gorm.io/gorm"
)

type {{.NameCamelLow}}RepositoryImpl struct {
	db *gorm.DB
}

// New{{.NameCamel}}Repository 创建 Repository 实例
func New{{.NameCamel}}Repository(db *gorm.DB) {{.NameCamel}}Repository {
	return &{{.NameCamelLow}}RepositoryImpl{db: db}
}

// Create 创建记录
func (r *{{.NameCamelLow}}RepositoryImpl) Create(ctx context.Context, {{.NameCamelLow}} *model.{{.NameCamel}}) error {
	return r.db.WithContext(ctx).Create({{.NameCamelLow}}).Error
}

// Update 更新记录
func (r *{{.NameCamelLow}}RepositoryImpl) Update(ctx context.Context, {{.NameCamelLow}} *model.{{.NameCamel}}) error {
	return r.db.WithContext(ctx).Save({{.NameCamelLow}}).Error
}

// Delete 删除记录（软删除）
func (r *{{.NameCamelLow}}RepositoryImpl) Delete(ctx context.Context, id {{.PrimaryKey.Type}}) error {
	return r.db.WithContext(ctx).Delete(&model.{{.NameCamel}}{}, id).Error
}

// ForceDelete 永久删除记录
func (r *{{.NameCamelLow}}RepositoryImpl) ForceDelete(ctx context.Context, id {{.PrimaryKey.Type}}) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.{{.NameCamel}}{}, id).Error
}

// GetByID 根据主键查询
func (r *{{.NameCamelLow}}RepositoryImpl) GetByID(ctx context.Context, id {{.PrimaryKey.Type}}) (*model.{{.NameCamel}}, error) {
	var {{.NameCamelLow}} model.{{.NameCamel}}
	err := r.db.WithContext(ctx).First(&{{.NameCamelLow}}, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &{{.NameCamelLow}}, nil
}

// List 分页查询
func (r *{{.NameCamelLow}}RepositoryImpl) List(ctx context.Context, page, pageSize int) ([]*model.{{.NameCamel}}, int64, error) {
	var list []*model.{{.NameCamel}}
	var total int64

	db := r.db.WithContext(ctx).Model(&model.{{.NameCamel}}{})

	// 查询总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListByCondition 条件查询
func (r *{{.NameCamelLow}}RepositoryImpl) ListByCondition(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*model.{{.NameCamel}}, int64, error) {
	var list []*model.{{.NameCamel}}
	var total int64

	db := r.db.WithContext(ctx).Model(&model.{{.NameCamel}}{})

	// 应用条件
	for key, value := range conditions {
		db = db.Where(key, value)
	}

	// 查询总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// Exists 检查记录是否存在
func (r *{{.NameCamelLow}}RepositoryImpl) Exists(ctx context.Context, id {{.PrimaryKey.Type}}) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.{{.NameCamel}}{}).Where("{{.PrimaryKey.Name}} = ?", id).Count(&count).Error
	return count > 0, err
}

// Count 统计总数
func (r *{{.NameCamelLow}}RepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.{{.NameCamel}}{}).Count(&count).Error
	return count, err
}

// BatchCreate 批量创建
func (r *{{.NameCamelLow}}RepositoryImpl) BatchCreate(ctx context.Context, {{.NameCamelLow}}s []*model.{{.NameCamel}}) error {
	return r.db.WithContext(ctx).CreateInBatches({{.NameCamelLow}}s, 100).Error
}
`

	return generateFile(cfg.Output+"/repository/"+table.Name+"_repository_impl.go", tmpl, table)
}

// 生成 service 接口
func generateServiceInterface(cfg *Config, table *TableInfo) error {
	tmpl := `package service

import (
	"context"
	"{{.Module}}/gen/model"
	"{{.Module}}/gen/dto"
)

// {{.NameCamel}}Service 定义业务逻辑接口
type {{.NameCamel}}Service interface {
	// Create{{.NameCamel}} 创建{{.NameCamel}}
	Create{{.NameCamel}}(ctx context.Context, req *dto.Create{{.NameCamel}}Request) (*model.{{.NameCamel}}, error)

	// Update{{.NameCamel}} 更新{{.NameCamel}}
	Update{{.NameCamel}}(ctx context.Context, id {{.PrimaryKey.Type}}, req *dto.Update{{.NameCamel}}Request) (*model.{{.NameCamel}}, error)

	// Delete{{.NameCamel}} 删除{{.NameCamel}}
	Delete{{.NameCamel}}(ctx context.Context, id {{.PrimaryKey.Type}}) error

	// Get{{.NameCamel}}ByID 根据ID获取{{.NameCamel}}
	Get{{.NameCamel}}ByID(ctx context.Context, id {{.PrimaryKey.Type}}) (*model.{{.NameCamel}}, error)

	// List{{.NameCamel}} 分页获取{{.NameCamel}}列表
	List{{.NameCamel}}(ctx context.Context, req *dto.List{{.NameCamel}}Request) (*dto.List{{.NameCamel}}Response, error)
}
`

	return generateFile(cfg.Output+"/service/"+table.Name+"_service.go", tmpl, table)
}

// 生成 service 实现
func generateServiceImpl(cfg *Config, table *TableInfo) error {
	tmpl := `package service

import (
	"context"
	"fmt"

	"{{.Module}}/gen/model"
	"{{.Module}}/gen/repository"
	"{{.Module}}/gen/dto"
)

type {{.NameCamelLow}}ServiceImpl struct {
	repo repository.{{.NameCamel}}Repository
}

// New{{.NameCamel}}Service 创建 Service 实例
func New{{.NameCamel}}Service(repo repository.{{.NameCamel}}Repository) {{.NameCamel}}Service {
	return &{{.NameCamelLow}}ServiceImpl{
		repo: repo,
	}
}

// Create{{.NameCamel}} 创建{{.NameCamel}}
func (s *{{.NameCamelLow}}ServiceImpl) Create{{.NameCamel}}(ctx context.Context, req *dto.Create{{.NameCamel}}Request) (*model.{{.NameCamel}}, error) {
	// 参数验证
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// TODO: 添加业务逻辑验证
	{{.NameCamelLow}} := &model.{{.NameCamel}}{
{{- range .Columns}}
{{- if and (not .IsPrimary) (not .IsAutoInc)}}
		{{.NameCamel}}: req.{{.NameCamel}},
{{- end}}
{{- end}}
	}

	if err := s.repo.Create(ctx, {{.NameCamelLow}}); err != nil {
		return nil, fmt.Errorf("创建失败: %v", err)
	}

	return {{.NameCamelLow}}, nil
}

// Update{{.NameCamel}} 更新{{.NameCamel}}
func (s *{{.NameCamelLow}}ServiceImpl) Update{{.NameCamel}}(ctx context.Context, id {{.PrimaryKey.Type}}, req *dto.Update{{.NameCamel}}Request) (*model.{{.NameCamel}}, error) {
	// 参数验证
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// 检查记录是否存在
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("{{.NameCamel}} 不存在")
	}

	// TODO: 添加业务逻辑验证
{{- range .Columns}}
{{- if and (not .IsPrimary) (not .IsAutoInc)}}
	if req.{{.NameCamel}} != nil {
		existing.{{.NameCamel}} = *req.{{.NameCamel}}
	}
{{- end}}
{{- end}}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("更新失败: %v", err)
	}

	return existing, nil
}

// Delete{{.NameCamel}} 删除{{.NameCamel}}
func (s *{{.NameCamelLow}}ServiceImpl) Delete{{.NameCamel}}(ctx context.Context, id {{.PrimaryKey.Type}}) error {
	// 检查记录是否存在
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}
	if !exists {
		return fmt.Errorf("{{.NameCamel}} 不存在")
	}

	// TODO: 添加业务逻辑验证（如检查是否存在关联数据）

	return s.repo.Delete(ctx, id)
}

// Get{{.NameCamel}}ByID 根据ID获取{{.NameCamel}}
func (s *{{.NameCamelLow}}ServiceImpl) Get{{.NameCamel}}ByID(ctx context.Context, id {{.PrimaryKey.Type}}) (*model.{{.NameCamel}}, error) {
	{{.NameCamelLow}}, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	if {{.NameCamelLow}} == nil {
		return nil, fmt.Errorf("{{.NameCamel}} 不存在")
	}
	return {{.NameCamelLow}}, nil
}

// List{{.NameCamel}} 分页获取{{.NameCamel}}列表
func (s *{{.NameCamelLow}}ServiceImpl) List{{.NameCamel}}(ctx context.Context, req *dto.List{{.NameCamel}}Request) (*dto.List{{.NameCamel}}Response, error) {
	// 参数验证和默认值
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("参数验证失败: %v", err)
	}

	// TODO: 构建查询条件
	conditions := make(map[string]interface{})
	// 示例：if req.Status != nil {
	//     conditions["status"] = *req.Status
	// }

	list, total, err := s.repo.ListByCondition(ctx, conditions, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}

	return &dto.List{{.NameCamel}}Response{
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
		Data:  list,
	}, nil
}
`

	return generateFile(cfg.Output+"/service/"+table.Name+"_service_impl.go", tmpl, table)
}

// 生成 DTO
func generateDTO(cfg *Config, table *TableInfo) error {
	tmpl := `package dto

import (
	"fmt"
	"{{.Module}}/gen/model"
)

// Create{{.NameCamel}}Request 创建请求
type Create{{.NameCamel}}Request struct {
{{- range .Columns}}
{{- if and (not .IsPrimary) (not .IsAutoInc)}}
	{{.NameCamel}} {{.Type}} ` + "`json:\"{{.Name}}\" binding:\"{{if eq .Type \"string\"}}required{{end}}\"`" + `
{{- end}}
{{- end}}
}

// Validate 验证请求参数
func (r *Create{{.NameCamel}}Request) Validate() error {
	// TODO: 添加自定义验证逻辑
	return nil
}

// Update{{.NameCamel}}Request 更新请求
type Update{{.NameCamel}}Request struct {
{{- range .Columns}}
{{- if and (not .IsPrimary) (not .IsAutoInc)}}
	{{.NameCamel}} *{{.Type}} ` + "`json:\"{{.Name}}\"`" + `
{{- end}}
{{- end}}
}

// Validate 验证请求参数
func (r *Update{{.NameCamel}}Request) Validate() error {
	// TODO: 添加自定义验证逻辑
	return nil
}

// List{{.NameCamel}}Request 列表查询请求
type List{{.NameCamel}}Request struct {
	Page     int ` + "`json:\"page\" form:\"page\"`" + `
	PageSize int ` + "`json:\"page_size\" form:\"page_size\"`" + `
	// TODO: 添加查询条件字段
	// Status  *int    ` + "`json:\"status\" form:\"status\"`" + `
	// Keyword *string ` + "`json:\"keyword\" form:\"keyword\"`" + `
}

// Validate 验证请求参数
func (r *List{{.NameCamel}}Request) Validate() error {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 10
	}
	return nil
}

// List{{.NameCamel}}Response 列表查询响应
type List{{.NameCamel}}Response struct {
	Total int64                ` + "`json:\"total\"`" + `
	Page  int                  ` + "`json:\"page\"`" + `
	Size  int                  ` + "`json:\"size\"`" + `
	Data  []*model.{{.NameCamel}} ` + "`json:\"data\"`" + `
}
`

	return generateFile(cfg.Output+"/dto/"+table.Name+"_dto.go", tmpl, table)
}

// 生成 Handler
func generateHandler(cfg *Config, table *TableInfo) error {
	tmpl := `package handler

import (
	"net/http"
	"strconv"

	"{{.Module}}/gen/dto"
	"{{.Module}}/gen/service"
	"github.com/gin-gonic/gin"
)

type {{.NameCamel}}Handler struct {
	{{.NameCamelLow}}Service service.{{.NameCamel}}Service
}

func New{{.NameCamel}}Handler({{.NameCamelLow}}Service service.{{.NameCamel}}Service) *{{.NameCamel}}Handler {
	return &{{.NameCamel}}Handler{
		{{.NameCamelLow}}Service: {{.NameCamelLow}}Service,
	}
}

// Create 创建{{.NameCamel}}
// @Summary 创建{{.NameCamel}}
// @Tags {{.NameCamel}}
// @Accept json
// @Produce json
// @Param request body dto.Create{{.NameCamel}}Request true "请求参数"
// @Success 200 {object} model.{{.NameCamel}}
// @Router /api/v1/{{.NameCamelLow}} [post]
func (h *{{.NameCamel}}Handler) Create(c *gin.Context) {
	var req dto.Create{{.NameCamel}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	{{.NameCamelLow}}, err := h.{{.NameCamelLow}}Service.Create{{.NameCamel}}(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, {{.NameCamelLow}})
}

// Update 更新{{.NameCamel}}
// @Summary 更新{{.NameCamel}}
// @Tags {{.NameCamel}}
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param request body dto.Update{{.NameCamel}}Request true "请求参数"
// @Success 200 {object} model.{{.NameCamel}}
// @Router /api/v1/{{.NameCamelLow}}/{id} [put]
func (h *{{.NameCamel}}Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req dto.Update{{.NameCamel}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	{{.NameCamelLow}}, err := h.{{.NameCamelLow}}Service.Update{{.NameCamel}}(c.Request.Context(), {{.PrimaryKey.Type}}(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, {{.NameCamelLow}})
}

// Delete 删除{{.NameCamel}}
// @Summary 删除{{.NameCamel}}
// @Tags {{.NameCamel}}
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/{{.NameCamelLow}}/{id} [delete]
func (h *{{.NameCamel}}Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.{{.NameCamelLow}}Service.Delete{{.NameCamel}}(c.Request.Context(), {{.PrimaryKey.Type}}(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetByID 获取{{.NameCamel}}详情
// @Summary 获取{{.NameCamel}}详情
// @Tags {{.NameCamel}}
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} model.{{.NameCamel}}
// @Router /api/v1/{{.NameCamelLow}}/{id} [get]
func (h *{{.NameCamel}}Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	{{.NameCamelLow}}, err := h.{{.NameCamelLow}}Service.Get{{.NameCamel}}ByID(c.Request.Context(), {{.PrimaryKey.Type}}(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, {{.NameCamelLow}})
}

// List 获取{{.NameCamel}}列表
// @Summary 获取{{.NameCamel}}列表
// @Tags {{.NameCamel}}
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} dto.List{{.NameCamel}}Response
// @Router /api/v1/{{.NameCamelLow}} [get]
func (h *{{.NameCamel}}Handler) List(c *gin.Context) {
	var req dto.List{{.NameCamel}}Request
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.{{.NameCamelLow}}Service.List{{.NameCamel}}(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
`

	return generateFile(cfg.Output+"/handler/"+table.Name+"_handler.go", tmpl, table)
}

func generateFile(path, tmplContent string, data interface{}) error {
	return generateFileWithFuncMap(path, tmplContent, data, nil)
}

func generateFileWithFuncMap(path, tmplContent string, data interface{}, funcMap template.FuncMap) error {
	// 创建目录
	dir := strings.Split(path, "/")
	dirPath := strings.Join(dir[:len(dir)-1], "/")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	// 创建文件
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 解析模板
	tmpl := template.New("template")
	if funcMap != nil {
		tmpl = tmpl.Funcs(funcMap)
	}
	tmpl, err = tmpl.Parse(tmplContent)
	if err != nil {
		return err
	}

	// 执行模板
	return tmpl.Execute(file, data)
}