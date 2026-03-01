package service

import (
	"fmt"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

// ProblemService 题目服务接口
type ProblemService interface {
	GetProblems(userID string) ([]*models.Problem, error)
	GetProblemDetail(problemID int, userID string) (*models.Problem, error)
}

// ProblemServiceImpl 题目服务实现
type ProblemServiceImpl struct {
	problemRepo *repository.ProblemRepository
}

// NewProblemService 创建题目服务实例
func NewProblemService(problemRepo *repository.ProblemRepository) *ProblemServiceImpl {
	return &ProblemServiceImpl{problemRepo: problemRepo}
}

// GetProblems 获取题目列表
func (s *ProblemServiceImpl) GetProblems(userID string) ([]*models.Problem, error) {
	var problems []*models.Problem
	var err error

	if userID == "" {
		// 未登录用户，只获取基础题目信息
		problems, err = s.problemRepo.GetProblems()
		if err != nil {
			return nil, fmt.Errorf("获取题目列表失败: %w", err)
		}
	} else {
		// 已登录用户，获取包含解题状态的题目列表
		problems, err = s.problemRepo.GetProblemsWithStatus(userID)
		if err != nil {
			return nil, fmt.Errorf("获取题目列表失败: %w", err)
		}
	}

	return problems, nil
}

// GetProblemDetail 获取题目详情
func (s *ProblemServiceImpl) GetProblemDetail(problemID int, userID string) (*models.Problem, error) {
	// 获取题目基础信息
	problem, err := s.problemRepo.GetProblemByID(problemID)
	if err != nil {
		return nil, fmt.Errorf("获取题目详情失败: %w", err)
	}

	// 如果用户已登录，检查是否已解决
	if userID != "" {
		problems, err := s.problemRepo.GetProblemsWithStatus(userID)
		if err == nil {
			for _, p := range problems {
				if p.ID == problemID {
					problem.IsSolved = p.IsSolved
					break
				}
			}
		}
	}

	return problem, nil
}