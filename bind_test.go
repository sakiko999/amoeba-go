// BindAs 在绑定时同步校验服务全部导出方法的参数/返回类型(非法即返回错误、
// BindAs panic)。main 绑定 Search 服务前先在这里走同一校验路径,提前暴露
// *mygo.Channel 参数或 Item.RawMessage 类型不被 IPC 层接受的问题,无需起 GUI。

package main

import (
	"testing"

	"github.com/egoist/mygo"

	"mygo-host/components/player"
	"mygo-host/components/profile"
	"mygo-host/components/search"
)

func TestBindSearchService(t *testing.T) {
	svc, _, err := search.NewService()
	if err != nil {
		t.Fatal(err)
	}
	mygo.BindAs("Search", svc)
}

type stubOpener struct{}

func (stubOpener) OpenPlayerH5(string, player.H5Target) error { return nil }
func (stubOpener) Target(string) (player.H5Target, bool)      { return player.H5Target{}, false }

type stubProfileOpener struct{}

func (stubProfileOpener) OpenProfile(string, profile.ProfileTarget) error { return nil }
func (stubProfileOpener) ProfileTarget(string) (profile.ProfileTarget, bool) {
	return profile.ProfileTarget{}, false
}

// Player.Target 的返回类型(H5Target 含 platform.Stream 数组)必须过 BindAs 的
// JSON 校验 —— 页面经 Player.Target 取播放目标,绑定失败等于整个 H5 播放不可用。
func TestBindPlayerService(t *testing.T) {
	svc, _, err := player.NewService(stubOpener{})
	if err != nil {
		t.Fatal(err)
	}
	mygo.BindAs("Player", svc)
}

// Profile 的方法参数含 platform.Item(OpenParams)、返回含 Item 数组,必须过
// BindAs 的 JSON 校验 —— 绑定失败等于个人主页整页不可用。
func TestBindProfileService(t *testing.T) {
	svc, _, err := profile.NewService(stubProfileOpener{})
	if err != nil {
		t.Fatal(err)
	}
	mygo.BindAs("Profile", svc)
}
