import React, { useState, useCallback } from 'react'
import Cropper, { Area, Point } from 'react-easy-crop'

interface ImageCropperProps {
    image: string
    aspect: number // e.g. 1 / 1 or 4 / 3
    onCropComplete: (croppedImage: Blob) => void
    onCancel: () => void
}

export const getCroppedImg = async (
    imageSrc: string,
    pixelCrop: Area,
    rotation = 0
): Promise<Blob> => {
    const image = new Image()
    image.src = imageSrc
    await new Promise((resolve) => (image.onload = resolve))

    const canvas = document.createElement('canvas')
    const ctx = canvas.getContext('2d')

    if (!ctx) {
        throw new Error('No 2d context')
    }

    // Set canvas size to the cropped area
    canvas.width = pixelCrop.width
    canvas.height = pixelCrop.height

    // Draw the image on the canvas
    ctx.drawImage(
        image,
        pixelCrop.x,
        pixelCrop.y,
        pixelCrop.width,
        pixelCrop.height,
        0,
        0,
        pixelCrop.width,
        pixelCrop.height
    )

    return new Promise((resolve, reject) => {
        canvas.toBlob((blob) => {
            if (!blob) {
                reject(new Error('Canvas is empty'))
                return
            }
            resolve(blob)
        }, 'image/jpeg')
    })
}

export default function ImageCropper({ image, aspect, onCropComplete, onCancel }: ImageCropperProps) {
    const [crop, setCrop] = useState<Point>({ x: 0, y: 0 })
    const [zoom, setZoom] = useState(1)
    const [croppedAreaPixels, setCroppedAreaPixels] = useState<Area | null>(null)

    const onCropChange = (crop: Point) => {
        setCrop(crop)
    }

    const onZoomChange = (zoom: number) => {
        setZoom(zoom)
    }

    const onCropCompleteInternal = useCallback((_croppedArea: Area, croppedAreaPixels: Area) => {
        setCroppedAreaPixels(croppedAreaPixels)
    }, [])

    const handleCrop = async () => {
        try {
            if (croppedAreaPixels) {
                const croppedImage = await getCroppedImg(image, croppedAreaPixels)
                onCropComplete(croppedImage)
            }
        } catch (e) {
            console.error(e)
        }
    }

    return (
        <div className="cropper-container">
            <div className="cropper-wrapper">
                <Cropper
                    image={image}
                    crop={crop}
                    zoom={zoom}
                    aspect={aspect}
                    onCropChange={onCropChange}
                    onCropComplete={onCropCompleteInternal}
                    onZoomChange={onZoomChange}
                />
            </div>
            <div className="controls">
                <div className="zoom-control">
                    <span>缩放:</span>
                    <input
                        type="range"
                        value={zoom}
                        min={1}
                        max={3}
                        step={0.1}
                        aria-labelledby="Zoom"
                        onChange={(e) => onZoomChange(Number(e.target.value))}
                        className="zoom-range"
                    />
                </div>
                <div className="actions">
                    <button onClick={onCancel} className="btn-cancel">
                        取消
                    </button>
                    <button onClick={handleCrop} className="btn-confirm">
                        确认裁剪
                    </button>
                </div>
            </div>

            <style>{`
        .cropper-container {
          position: fixed;
          top: 0;
          left: 0;
          right: 0;
          bottom: 0;
          background-color: rgba(0, 0, 0, 0.8);
          z-index: 9999;
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
        }

        .cropper-wrapper {
          position: relative;
          width: 100%;
          height: 70vh;
          max-width: 600px;
          background: #333;
        }

        .controls {
          width: 100%;
          max-width: 600px;
          padding: 1rem;
          background: #fff;
          display: flex;
          flex-direction: column;
          gap: 1rem;
          border-radius: 0 0 8px 8px;
        }

        .zoom-control {
          display: flex;
          align-items: center;
          gap: 1rem;
          color: #374151;
          font-size: 14px;
        }

        .zoom-range {
          flex: 1;
        }

        .actions {
          display: flex;
          justify-content: flex-end;
          gap: 1rem;
        }

        .btn-cancel,
        .btn-confirm {
          padding: 0.5rem 1.5rem;
          border: none;
          border-radius: 6px;
          font-size: 14px;
          cursor: pointer;
          transition: all 0.2s;
        }

        .btn-cancel {
          background: #f3f4f6;
          color: #374151;
        }

        .btn-confirm {
          background: #3b82f6;
          color: white;
        }

        .btn-confirm:hover {
          background: #2563eb;
        }
      `}</style>
        </div>
    )
}
