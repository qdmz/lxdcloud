package nginx

import _ "embed"

// defaultNginxTemplate 内置默认站点模板；工作目录存在 nginx-default.tmpl 时优先使用文件
//
//go:embed nginx-default.tmpl
var defaultNginxTemplate string
