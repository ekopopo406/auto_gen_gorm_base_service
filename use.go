package main

import (
    "log"
    
    "github.com/gin-gonic/gin"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    
    "github.com/yourusername/yourproject/gen/handler"
    "github.com/yourusername/yourproject/gen/repository"
    "github.com/yourusername/yourproject/gen/service"
)

func main() {
    // 初始化 GORM
    dsn := "root:password@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }
    
    // 初始化 Repository、Service、Handler
    userRepo := repository.NewUserRepository(db)
    userService := service.NewUserService(userRepo)
    userHandler := handler.NewUserHandler(userService)
    
    // 设置路由
    r := gin.Default()
    
    api := r.Group("/api/v1/users")
    {
        api.POST("", userHandler.Create)
        api.PUT("/:id", userHandler.Update)
        api.DELETE("/:id", userHandler.Delete)
        api.GET("/:id", userHandler.GetByID)
        api.GET("", userHandler.List)
    }
    
    r.Run(":8080")
}