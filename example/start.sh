#!/bin/bash

echo "======================================"
echo "  IM + User 集成示例"
echo "======================================"
echo ""

# 检查数据库
echo "检查数据库连接..."
if ! mysql -uroot -pyyy003014 -e "USE im_user_test;" 2>/dev/null; then
    echo "数据库不存在，正在初始化..."
    cd sql && ./init_db.sh && cd ..
else
    echo "数据库已存在"
fi

echo ""
echo "启动服务器..."
echo ""

# 启动服务
go run main.go

# 或者编译后运行
# go build -o server
# ./server
