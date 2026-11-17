// BindAs 在绑定时同步校验服务全部导出方法的参数/返回类型(非法即返回错误、
// BindAs panic)。main 绑定 Search 服务前先在这里走同一校验路径,提前暴露
// *mygo.Channel 参数或 Item.RawMessage 类型不被 IPC 层接受的问题,无需起 GUI。

package main

import (
	"testing"

	"github.com/egoist/mygo"

	"mygo-host/search"
)

func TestBindSearchService(t *testing.T) {
	mygo.BindAs("Search", search.NewService(nil))
}
