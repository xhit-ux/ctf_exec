import React, { useState, useRef } from 'react'
import ImageCropper from './ImageCropper'
import { apiUrl, staticUrl } from '../../utils/api'
import { fromUnknownError, readSafeErrorMessage } from '../../utils/error'

interface AvatarUploadProps {
  currentAvatarUrl?: string
  onUploadSuccess?: (newUrl: string) => void
}

export default function AvatarUpload({ currentAvatarUrl, onUploadSuccess }: AvatarUploadProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [tempImageUrl, setTempImageUrl] = useState<string | null>(null)
  const [showCropper, setShowCropper] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string>(currentAvatarUrl || '')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string>('')
  const [success, setSuccess] = useState<string>('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    // 验证文件类型
    const allowedTypes = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif', 'image/webp']
    if (!allowedTypes.includes(file.type)) {
      setError('仅支持 JPG、PNG、GIF、WebP 格式的图片')
      return
    }

    // 验证文件大小 (2MB)
    if (file.size > 2 * 1024 * 1024) {
      setError('文件大小不能超过 2MB')
      return
    }

    setError('')

    // 读取文件展示到裁剪框
    const reader = new FileReader()
    reader.onload = () => {
      setTempImageUrl(reader.result as string)
      setShowCropper(true)
    }
    reader.readAsDataURL(file)
  }

  const handleCropComplete = async (croppedBlob: Blob) => {
    setShowCropper(false)
    const croppedFile = new File([croppedBlob], 'avatar.jpg', {
      type: 'image/jpeg',
    })
    setSelectedFile(croppedFile)

    // 生成预览
    const reader = new FileReader()
    reader.onloadend = () => {
      setPreviewUrl(reader.result as string)
    }
    reader.readAsDataURL(croppedBlob)
  }

  const handleUpload = async () => {
    if (!selectedFile) {
      setError('请先选择图片')
      return
    }

    setUploading(true)
    setError('')
    setSuccess('')

    try {
      const formData = new FormData()
      formData.append('avatar', selectedFile)

      const token = localStorage.getItem('token')
      const response = await fetch(apiUrl('/api/upload/avatar'), {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
        body: formData,
      })

      if (!response.ok) {
        setError(await readSafeErrorMessage(response, '上传失败，请稍后重试'))
        return
      }

      const data = await response.json()
      setSuccess('头像上传成功！')
      setSelectedFile(null)

      if (onUploadSuccess) {
        onUploadSuccess(data.avatar_url)
      }
    } catch (err) {
      setError(fromUnknownError(err, '上传失败，请稍后重试'))
    } finally {
      setUploading(false)
    }
  }

  const handleClickUpload = () => {
    fileInputRef.current?.click()
  }

  // 格式化预览 URL，如果是 static 开头则拼上后端地址
  const displayPreviewUrl = staticUrl(previewUrl)

  return (
    <div className="avatar-upload">
      <div className="avatar-preview">
        {previewUrl ? (
          <img src={displayPreviewUrl} alt="头像预览" className="avatar-image" />
        ) : (
          <div className="avatar-placeholder">
            <span>暂无头像</span>
          </div>
        )}
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/jpg,image/png,image/gif,image/webp"
        onChange={handleFileSelect}
        style={{ display: 'none' }}
        value="" // 允许重复选择同一个文件
      />

      <div className="upload-actions">
        <button onClick={handleClickUpload} className="btn-select" disabled={uploading}>
          选择图片并裁剪
        </button>
        {selectedFile && (
          <button onClick={handleUpload} className="btn-upload" disabled={uploading}>
            {uploading ? '上传中...' : '开始上传'}
          </button>
        )}
      </div>

      {error && <div className="error-message">{error}</div>}
      {success && <div className="success-message">{success}</div>}

      {showCropper && tempImageUrl && (
        <ImageCropper
          image={tempImageUrl}
          aspect={1 / 1}
          onCropComplete={handleCropComplete}
          onCancel={() => setShowCropper(false)}
        />
      )}

      <div className="upload-tips">
        <p>• 裁剪比例固定为 1:1</p>
        <p>• 支持 JPG、PNG、GIF、WebP 格式</p>
        <p>• 文件大小不超过 2MB</p>
      </div>

      <style>{`
        .avatar-upload {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 1rem;
          padding: 1.5rem;
          border: 1px solid #e5e7eb;
          border-radius: 8px;
          background: #fff;
        }

        .avatar-preview {
          width: 150px;
          height: 150px;
          border-radius: 50%;
          overflow: hidden;
          border: 3px solid #e5e7eb;
        }

        .avatar-image {
          width: 100%;
          height: 100%;
          object-fit: cover;
        }

        .avatar-placeholder {
          width: 100%;
          height: 100%;
          display: flex;
          align-items: center;
          justify-content: center;
          background: #f3f4f6;
          color: #9ca3af;
          font-size: 14px;
        }

        .upload-actions {
          display: flex;
          gap: 0.5rem;
        }

        .btn-select,
        .btn-upload {
          padding: 0.5rem 1rem;
          border: none;
          border-radius: 6px;
          font-size: 14px;
          cursor: pointer;
          transition: all 0.2s;
        }

        .btn-select {
          background: #f3f4f6;
          color: #374151;
        }

        .btn-select:hover:not(:disabled) {
          background: #e5e7eb;
        }

        .btn-upload {
          background: #3b82f6;
          color: white;
        }

        .btn-upload:hover:not(:disabled) {
          background: #2563eb;
        }

        .btn-select:disabled,
        .btn-upload:disabled {
          opacity: 0.5;
          cursor: not-allowed;
        }

        .error-message {
          color: #ef4444;
          font-size: 14px;
          padding: 0.5rem 1rem;
          background: #fee2e2;
          border-radius: 6px;
        }

        .success-message {
          color: #10b981;
          font-size: 14px;
          padding: 0.5rem 1rem;
          background: #d1fae5;
          border-radius: 6px;
        }

        .upload-tips {
          font-size: 12px;
          color: #6b7280;
          text-align: center;
        }

        .upload-tips p {
          margin: 0.25rem 0;
        }
      `}</style>
    </div>
  )
}
