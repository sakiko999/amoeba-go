// BindAs 在绑定时同步校验服务全部导出方法的参数/返回类型(非法即返回错误、
// BindAs panic)。main 绑定 Search 服务前先在这里走同一校验路径,提前暴露
// *mygo.Channel 参数或 Item.RawMessage 类型不被 IPC 层接受的问题,无需起 GUI。

package main

import (
	"testing"

	"github.com/egoist/mygo"

	"mygo-host/components/player"
	"mygo-host/search"
)

func TestBindSearchService(t *testing.T) {
	mygo.BindAs("Search", search.NewService(nil))
}

type stubOpener struct{}

func (stubOpener) OpenPlayer(string, string, string) error    { return nil }
func (stubOpener) OpenPlayerH5(string, player.H5Target) error { return nil }
func (stubOpener) Target(string) (player.H5Target, bool)      { return player.H5Target{}, false }

// Player.Target 的返回类型(H5Target 含 platform.Stream 数组)必须过 BindAs 的
// JSON 校验 —— 页面经 Player.Target 取播放目标,绑定失败等于整个 H5 播放不可用。
func TestBindPlayerService(t *testing.T) {
	svc, _, err := player.NewService(stubOpener{})
	if err != nil {
		t.Fatal(err)
	}
	mygo.BindAs("Player", svc)
}
