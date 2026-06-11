# 🐍 贪吃蛇小游戏

一个简单、美观的贪吃蛇网页游戏。

## 🎮 游戏特点

- 🎯 经典贪吃蛇玩法
- 🌈 精美的渐变色彩效果
- 📱 响应式设计，支持移动端
- 💾 本地存储最高分记录
- ⚡ 速度随分数增加而提升

## 🚀 如何运行

### 方法一：直接打开
1. 双击 `snake_game.html` 文件
2. 在浏览器中打开即可开始游戏

### 方法二：使用Python服务器（推荐）
```bash
# 给脚本添加执行权限
chmod +x start_snake_game.py

# 运行启动脚本
python3 start_snake_game.py
```

然后访问：http://localhost:8000/snake_game.html

### 方法三：使用其他HTTP服务器
```bash
# 使用Python内置HTTP服务器
python3 -m http.server 8000

# 或使用Node.js的http-server
npx http-server -p 8000
```

## 🎯 游戏操作

- **方向键 ↑ ↓ ← →**：控制蛇的移动方向
- **空格键**：暂停/继续游戏
- **开始游戏按钮**：开始新游戏
- **暂停按钮**：暂停/继续游戏
- **重新开始按钮**：重置游戏

## 🏆 游戏规则

1. 控制蛇吃到红色食物获得10分
2. 每吃一个食物，蛇身变长，速度略微提升
3. 撞到墙壁或自己的身体游戏结束
4. 最高分记录会保存在浏览器中

## 🛠️ 技术栈

- HTML5 Canvas
- 纯JavaScript（无框架依赖）
- CSS3 渐变和动画效果
- LocalStorage 存储最高分

## 📁 文件结构

```
├── snake_game.html      # 游戏主文件
├── start_snake_game.py  # Python启动脚本
└── README.md           # 说明文档
```

## 🎨 自定义修改

如需修改游戏参数，可以编辑 `snake_game.html` 中的以下常量：

```javascript
const GRID_SIZE = 20;      // 网格大小
const CANVAS_SIZE = 400;   // 画布大小
const CELL_SIZE = CANVAS_SIZE / GRID_SIZE;  // 单元格大小
```

## 📝 许可证

MIT License