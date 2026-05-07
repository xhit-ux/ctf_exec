package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

// ============================================================
// problem_hints + hint_unlocks 提示子系统（v2.1 Part 6.1 / v3.0 Part 9）
// ============================================================

// ListProblemHints 列出某题目的所有提示。
// userID 非空时同时回填 is_unlocked；contestID 为 nil 表示 platform scope。
func (r *ProblemRepository) ListProblemHints(problemID, userID string, contestID *string) ([]*models.ProblemHint, error) {
	rows, err := r.db.Query(`
		SELECT id, problem_id, content, cost, sort_order, is_visible, created_at
		FROM problem_hints
		WHERE problem_id = ?
		ORDER BY sort_order ASC, created_at ASC
	`, strings.TrimSpace(problemID))
	if err != nil {
		return nil, fmt.Errorf("查询提示列表失败: %w", err)
	}
	defer rows.Close()

	hints := make([]*models.ProblemHint, 0)
	for rows.Next() {
		h := &models.ProblemHint{}
		if err := rows.Scan(&h.ID, &h.ProblemID, &h.Content, &h.Cost, &h.SortOrder, &h.IsVisible, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描提示失败: %w", err)
		}
		hints = append(hints, h)
	}

	if strings.TrimSpace(userID) != "" && len(hints) > 0 {
		unlocked, err := r.loadHintUnlocks(userID, problemID, contestID)
		if err != nil {
			return nil, err
		}
		for _, h := range hints {
			if _, ok := unlocked[h.ID]; ok {
				h.IsUnlocked = true
			}
		}
	}
	return hints, nil
}

// loadHintUnlocks 取该用户在指定 scope 下已解锁的 hint id 集合。
// platform scope: contest_id IS NULL；contest scope: contest_id = ?。
func (r *ProblemRepository) loadHintUnlocks(userID, problemID string, contestID *string) (map[string]struct{}, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if contestID == nil || strings.TrimSpace(*contestID) == "" {
		rows, err = r.db.Query(`
			SELECT u.hint_id
			FROM hint_unlocks u
			JOIN problem_hints h ON h.id = u.hint_id
			WHERE u.user_id = ? AND u.contest_id IS NULL AND h.problem_id = ?
		`, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	} else {
		rows, err = r.db.Query(`
			SELECT u.hint_id
			FROM hint_unlocks u
			JOIN problem_hints h ON h.id = u.hint_id
			WHERE u.user_id = ? AND u.contest_id = ? AND h.problem_id = ?
		`, strings.TrimSpace(userID), strings.TrimSpace(*contestID), strings.TrimSpace(problemID))
	}
	if err != nil {
		return nil, fmt.Errorf("查询已解锁提示失败: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var hid string
		if err := rows.Scan(&hid); err != nil {
			return nil, fmt.Errorf("扫描解锁记录失败: %w", err)
		}
		out[hid] = struct{}{}
	}
	return out, nil
}

// CreateHint 创建提示。
func (r *ProblemRepository) CreateHint(problemID string, req *models.CreateHintRequest) (*models.ProblemHint, error) {
	id := uuid.New().String()
	visible := true
	if req.IsVisible != nil {
		visible = *req.IsVisible
	}
	_, err := r.db.Exec(`
		INSERT INTO problem_hints (id, problem_id, content, cost, sort_order, is_visible)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, strings.TrimSpace(problemID), strings.TrimSpace(req.Content), req.Cost, req.SortOrder, visible)
	if err != nil {
		return nil, fmt.Errorf("创建提示失败: %w", err)
	}
	return r.GetHint(id)
}

// UpdateHint 更新提示。
func (r *ProblemRepository) UpdateHint(hintID string, req *models.UpdateHintRequest) error {
	visible := true
	if req.IsVisible != nil {
		visible = *req.IsVisible
	}
	res, err := r.db.Exec(`
		UPDATE problem_hints
		SET content = ?, cost = ?, sort_order = ?, is_visible = ?
		WHERE id = ?
	`, strings.TrimSpace(req.Content), req.Cost, req.SortOrder, visible, strings.TrimSpace(hintID))
	if err != nil {
		return fmt.Errorf("更新提示失败: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "提示不存在")
	}
	return nil
}

// DeleteHint 删除提示。
func (r *ProblemRepository) DeleteHint(hintID string) error {
	res, err := r.db.Exec(`DELETE FROM problem_hints WHERE id = ?`, strings.TrimSpace(hintID))
	if err != nil {
		return fmt.Errorf("删除提示失败: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "提示不存在")
	}
	return nil
}

// GetHint 单条提示查询。
func (r *ProblemRepository) GetHint(hintID string) (*models.ProblemHint, error) {
	h := &models.ProblemHint{}
	err := r.db.QueryRow(`
		SELECT id, problem_id, content, cost, sort_order, is_visible, created_at
		FROM problem_hints WHERE id = ?
	`, strings.TrimSpace(hintID)).Scan(&h.ID, &h.ProblemID, &h.Content, &h.Cost, &h.SortOrder, &h.IsVisible, &h.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "提示不存在")
		}
		return nil, fmt.Errorf("查询提示失败: %w", err)
	}
	return h, nil
}

// UnlockHint 解锁提示。
// 已解锁返回 (hint, false, nil)；新解锁返回 (hint, true, nil)。
// contestID 为 nil 表示 platform scope。
func (r *ProblemRepository) UnlockHint(userID, hintID string, contestID *string) (*models.ProblemHint, bool, error) {
	hint, err := r.GetHint(hintID)
	if err != nil {
		return nil, false, err
	}
	if !hint.IsVisible {
		return nil, false, apperr.New(apperr.ErrForbidden, "提示已隐藏")
	}

	var contestArg interface{}
	if contestID != nil && strings.TrimSpace(*contestID) != "" {
		contestArg = strings.TrimSpace(*contestID)
	} else {
		contestArg = nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, false, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 已解锁则直接返回。
	var existsCount int
	if contestArg == nil {
		err = tx.QueryRow(`SELECT COUNT(*) FROM hint_unlocks WHERE user_id = ? AND hint_id = ? AND contest_id IS NULL`,
			strings.TrimSpace(userID), strings.TrimSpace(hintID)).Scan(&existsCount)
	} else {
		err = tx.QueryRow(`SELECT COUNT(*) FROM hint_unlocks WHERE user_id = ? AND hint_id = ? AND contest_id = ?`,
			strings.TrimSpace(userID), strings.TrimSpace(hintID), contestArg).Scan(&existsCount)
	}
	if err != nil {
		return nil, false, fmt.Errorf("查询解锁状态失败: %w", err)
	}
	if existsCount > 0 {
		_ = tx.Commit()
		hint.IsUnlocked = true
		return hint, false, nil
	}

	id := uuid.New().String()
	if _, err = tx.Exec(`
		INSERT INTO hint_unlocks (id, user_id, hint_id, contest_id, cost_at_unlock)
		VALUES (?, ?, ?, ?, ?)
	`, id, strings.TrimSpace(userID), strings.TrimSpace(hintID), contestArg, hint.Cost); err != nil {
		return nil, false, fmt.Errorf("解锁提示失败: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("提交事务失败: %w", err)
	}
	hint.IsUnlocked = true
	return hint, true, nil
}

// ============================================================
// problem_prerequisites 前置题目子系统（v2.1 Part 6.2）
// ============================================================

// ListProblemPrerequisites 列出某题目的所有前置题目（带 is_solved）。
func (r *ProblemRepository) ListProblemPrerequisites(problemID, userID string) ([]*models.PrerequisiteRef, error) {
	rows, err := r.db.Query(`
		SELECT pr.prerequisite_id, p.title,
		       CASE WHEN ? = '' THEN 0
		            ELSE EXISTS(
		                SELECT 1 FROM submissions s
		                WHERE s.problem_id = pr.prerequisite_id
		                  AND s.user_id = ? AND s.is_correct = 1
		            )
		       END AS is_solved
		FROM problem_prerequisites pr
		JOIN problems p ON p.id = pr.prerequisite_id
		WHERE pr.problem_id = ?
		ORDER BY p.title ASC
	`, strings.TrimSpace(userID), strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return nil, fmt.Errorf("查询前置题目失败: %w", err)
	}
	defer rows.Close()

	out := make([]*models.PrerequisiteRef, 0)
	for rows.Next() {
		ref := &models.PrerequisiteRef{}
		var solved int
		if err := rows.Scan(&ref.ID, &ref.Title, &solved); err != nil {
			return nil, fmt.Errorf("扫描前置题目失败: %w", err)
		}
		ref.IsSolved = solved == 1
		out = append(out, ref)
	}
	return out, nil
}

// SetProblemPrerequisites 全量替换某题目的前置题目集合。会做循环依赖检测。
func (r *ProblemRepository) SetProblemPrerequisites(problemID string, prerequisiteIDs []string) error {
	cleanID := strings.TrimSpace(problemID)
	// 去重 + 自指过滤。
	seen := make(map[string]struct{})
	clean := make([]string, 0, len(prerequisiteIDs))
	for _, raw := range prerequisiteIDs {
		v := strings.TrimSpace(raw)
		if v == "" || v == cleanID {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		clean = append(clean, v)
	}

	// 循环依赖检测：从每个候选 prereq 出发反向遍历，若到达自身则拒绝。
	for _, pid := range clean {
		cyclic, err := r.hasPrerequisiteCycle(pid, cleanID)
		if err != nil {
			return err
		}
		if cyclic {
			return apperr.New(apperr.ErrBadRequest, "存在循环依赖")
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM problem_prerequisites WHERE problem_id = ?`, cleanID); err != nil {
		return fmt.Errorf("清空前置依赖失败: %w", err)
	}
	for _, pid := range clean {
		if _, err = tx.Exec(`INSERT INTO problem_prerequisites (problem_id, prerequisite_id) VALUES (?, ?)`, cleanID, pid); err != nil {
			return fmt.Errorf("写入前置依赖失败: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// hasPrerequisiteCycle 在 problem_prerequisites 图上做 DFS，
// 检测从 startID 出发能否到达 targetID（用于检测引入新依赖后是否成环）。
func (r *ProblemRepository) hasPrerequisiteCycle(startID, targetID string) (bool, error) {
	visited := make(map[string]struct{})
	stack := []string{startID}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == targetID {
			return true, nil
		}
		if _, ok := visited[cur]; ok {
			continue
		}
		visited[cur] = struct{}{}

		rows, err := r.db.Query(`SELECT prerequisite_id FROM problem_prerequisites WHERE problem_id = ?`, cur)
		if err != nil {
			return false, fmt.Errorf("DFS 查询前置依赖失败: %w", err)
		}
		for rows.Next() {
			var nxt string
			if scanErr := rows.Scan(&nxt); scanErr != nil {
				rows.Close()
				return false, fmt.Errorf("扫描依赖图失败: %w", scanErr)
			}
			stack = append(stack, nxt)
		}
		rows.Close()
	}
	return false, nil
}

// AreAllPrerequisitesSolvedByUser 用于 is_locked 计算：
// 当前用户是否已通过该题目的全部前置题目（platform scope）。
func (r *ProblemRepository) AreAllPrerequisitesSolvedByUser(problemID, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}
	var unsolved int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM problem_prerequisites pr
		WHERE pr.problem_id = ?
		  AND NOT EXISTS (
		      SELECT 1 FROM submissions s
		      WHERE s.problem_id = pr.prerequisite_id
		        AND s.user_id = ? AND s.is_correct = 1
		  )
	`, strings.TrimSpace(problemID), strings.TrimSpace(userID)).Scan(&unsolved)
	if err != nil {
		return false, fmt.Errorf("查询前置完成情况失败: %w", err)
	}
	return unsolved == 0, nil
}
