import React, { useState, useRef } from 'react'
import ImageCropper from './ImageCropper'
import { apiUrl, staticUrl } from '../../utils/api'
import { apiFetch } from '../../utils/authClient'
import { fromUnknownError, readSafeErrorMessage } from '../../utils/error'

interface ContestCoverUploadProps {
    currentCoverUrl?: string
    onUploadSuccess?: (newUrl: string) => void
}

export default function ContestCoverUpload({ currentCoverUrl, onUploadSuccess }: ContestCoverUploadProps) {
    const [selectedFile, setSelectedFile] = useState<File | null>(null)
    const [tempImageUrl, setTempImageUrl] = useState<string | null>(null)
    const [showCropper, setShowCropper] = useState(false)
    const [previewUrl, setPreviewUrl] = useState<string>(currentCoverUrl || '')
    const [uploading, setUploading] = useState(false)
    const [error, setError] = useState<string>('')
    const [success, setSuccess] = useState<string>('')
    const fileInputRef = useRef<HTMLInputElement>(null)

    const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return

        const allowedTypes = ['image/jpeg', 'image/jpg', 'image/png', 'image/webp']
        if (!allowedTypes.includes(file.type)) {
            setError('仅支持 JPG、PNG、WebP 格式的图片')
            return
        }

        if (file.size > 5 * 1024 * 1024) {
            setError('文件大小不能超过 5MB')
            return
        }

        setError('')

        const reader = new FileReader()
        reader.onload = () => {
            setTempImageUrl(reader.result as string)
            setShowCropper(true)
        }
        reader.readAsDataURL(file)
    }

    const handleCropComplete = async (croppedBlob: Blob) => {
        setShowCropper(false)
        const croppedFile = new File([croppedBlob], 'cover.jpg', {
            type: 'image/jpeg',
        })
        setSelectedFile(croppedFile)

        const reader = new FileReader()
        reader.onloadend = () => {
            setPreviewUrl(reader.result as string)
        }
        reader.readAsDataURL(croppedBlob)
    }

    const handleUpload = async () => {
        if (!selectedFile) {
            setError('请先选择并裁剪图片')
            return
        }

        setUploading(true)
        setError('')
        setSuccess('')

        try {
            const formData = new FormData()
            formData.append('cover', selectedFile)

            const response = await apiFetch(apiUrl('/api/upload/contest-cover'), {
                method: 'POST',
                body: formData,
            })

            if (!response.ok) {
                setError(await readSafeErrorMessage(response, '上传失败，请稍后重试'))
                return
            }

            const data = await response.json()
            setSuccess('封面上传成功！')
            setSelectedFile(null)

            if (onUploadSuccess) {
                onUploadSuccess(data.cover_url)
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

    const displayPreviewUrl = staticUrl(previewUrl)

    return (
        <div className="contest-cover-upload">
            <div className="cover-preview-wrapper">
                {previewUrl ? (
                    <img src={displayPreviewUrl} alt="封面预览" className="cover-image" />
                ) : (
                    <div className="cover-placeholder">
                        <span>暂无封面 (4:3)</span>
                    </div>
                )}
            </div>

            <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/jpg,image/png,image/webp"
                onChange={handleFileSelect}
                style={{ display: 'none' }}
                value=""
            />

            <div className="upload-actions">
                <button onClick={handleClickUpload} className="btn-select" disabled={uploading}>
                    选择封面图片
                </button>
                {selectedFile && (
                    <button onClick={handleUpload} className="btn-upload" disabled={uploading}>
                        {uploading ? '上传中...' : '提交上传'}
                    </button>
                )}
            </div>

            {error && <div className="error-message">{error}</div>}
            {success && <div className="success-message">{success}</div>}

            {showCropper && tempImageUrl && (
                <ImageCropper
                    image={tempImageUrl}
                    aspect={4 / 3}
                    onCropComplete={handleCropComplete}
                    onCancel={() => setShowCropper(false)}
                />
            )}

            <style>{`
        .contest-cover-upload {
          display: flex;
          flex-direction: column;
          gap: 1rem;
          padding: 1rem;
          border: 1px dashed #d1d5db;
          border-radius: 8px;
          background: #f9fafb;
        }

        .cover-preview-wrapper {
          width: 100%;
          aspect-ratio: 4 / 3;
          border-radius: 4px;
          overflow: hidden;
          background: #e5e7eb;
          border: 1px solid #e5e7eb;
        }

        .cover-image {
          width: 100%;
          height: 100%;
          object-fit: cover;
        }

        .cover-placeholder {
          width: 100%;
          height: 100%;
          display: flex;
          align-items: center;
          justify-content: center;
          color: #9ca3af;
          font-size: 14px;
        }

        .upload-actions {
          display: flex;
          gap: 0.5rem;
        }

        .btn-select,
        .btn-upload {
          flex: 1;
          padding: 0.6rem;
          border-radius: 6px;
          font-size: 14px;
          cursor: pointer;
          border: none;
        }

        .btn-select {
          background: #fff;
          border: 1px solid #d1d5db;
          color: #374151;
        }

        .btn-upload {
          background: #10b981;
          color: white;
        }

        .error-message {
          color: #ef4444;
          font-size: 13px;
          padding: 0.5rem;
          background: #fee2e2;
          border-radius: 4px;
        }

        .success-message {
          color: #059669;
          font-size: 13px;
          padding: 0.5rem;
          background: #d1fae5;
          border-radius: 4px;
        }
      `}</style>
        </div>
    )
}
