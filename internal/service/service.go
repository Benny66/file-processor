// Package service 是业务服务层，介于绑定方法（app.go）与 model 之间。
//
// 分层纪律（由 guardrails 的 AST 护栏强制）：
//   - app.go 的绑定方法不得直接 import gorm/database，必须经本层的 service。
//   - service 层可以访问数据库（持有 *gorm.DB），model 层是叶子（不 import service/database）。
//
// 后续业务能力（资产等）在各自的 service 文件里实现，并在此注入数据库连接。
package service

import (
	"context"
	"sync"

	"gorm.io/gorm"

	"file-processor/internal/config"
)

// Service 聚合所有业务服务，是绑定方法访问业务能力的唯一入口。
// 由启动时初始化并注入 App，App 不再直接持有 *gorm.DB。
type Service struct {
	db  *gorm.DB
	cfg *config.Config

	// historyRoot 暂存区根目录（ADR-0007：处理结果只进这里，下载是唯一落盘出口）。
	// 由组合根经 SetHistoryRoot 注入；未注入时批次拒绝开跑（fail-fast 而非静默跳过历史）。
	historyRoot string

	// 批次互斥状态：同一时间只允许一个批次；取消由 CancelBatch 触发。
	batchMu      sync.Mutex
	batchCancel  context.CancelFunc
	batchRunning bool
}

// New 创建服务聚合，注入数据库连接与配置。
func New(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// SetHistoryRoot 设置暂存区根目录（应用数据目录下的 history/，组合根注入）。
func (s *Service) SetHistoryRoot(dir string) {
	s.historyRoot = dir
}

// Close 释放底层数据库连接，供优雅关闭时调用。
func (s *Service) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
