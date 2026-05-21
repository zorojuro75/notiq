package postgres

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/zorojuro75/notiq/internal/domain/entity"
    domainrepo "github.com/zorojuro75/notiq/internal/domain/repository"
    "github.com/zorojuro75/notiq/internal/repository/models"
    "github.com/zorojuro75/notiq/pkg/apperror"
)

type jobRepository struct {
    db *gorm.DB
}

func NewJobRepository(db *gorm.DB) domainrepo.JobRepository {
    return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, job *entity.Job) error {
    if job.ID == uuid.Nil {
        job.ID = uuid.New()
    }
    m := models.FromJobEntity(job)
    if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
        return err
    }
    job.CreatedAt = m.CreatedAt
    job.UpdatedAt = m.UpdatedAt
    return nil
}

func (r *jobRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error) {
    var m models.Job
    err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, apperror.ErrJobNotFound
        }
        return nil, err
    }
    return m.ToEntity(), nil
}

func (r *jobRepository) List(ctx context.Context, filter entity.JobFilter, page, pageSize int) ([]*entity.Job, int64, error) {
    var ms []models.Job
    var total int64

    q := r.db.WithContext(ctx).Model(&models.Job{})

    if filter.Status != nil {
        q = q.Where("status = ?", string(*filter.Status))
    }
    if filter.Type != nil {
        q = q.Where("type = ?", string(*filter.Type))
    }

    if err := q.Count(&total).Error; err != nil {
        return nil, 0, err
    }

    offset := (page - 1) * pageSize
    if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&ms).Error; err != nil {
        return nil, 0, err
    }

    jobs := make([]*entity.Job, len(ms))
    for i, m := range ms {
        m := m
        jobs[i] = m.ToEntity()
    }
    return jobs, total, nil
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.JobStatus) error {
    result := r.db.WithContext(ctx).Model(&models.Job{}).
        Where("id = ?", id).
        Update("status", string(status))
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return apperror.ErrJobNotFound
    }
    return nil
}

func (r *jobRepository) UpdateRetryCount(ctx context.Context, id uuid.UUID, retryCount int) error {
    result := r.db.WithContext(ctx).Model(&models.Job{}).
        Where("id = ?", id).
        Update("retry_count", retryCount)
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return apperror.ErrJobNotFound
    }
    return nil
}