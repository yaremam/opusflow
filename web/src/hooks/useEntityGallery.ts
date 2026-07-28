import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { errorMessage, pollForArtResolution, type ArtStatus } from '../api/library'

// EntityGalleryConfig is the only thing that varies between an
// ArtistDetailPage and an AlbumDetailPage — fetch/retry/upload/set-primary/
// delete/remove all follow the identical shape (load on mount, mutate,
// replace local state with the server's response) for both. ArtSummary is
// the narrower type retryArt resolves to (Artist/Album, not the full
// Detail) — see retryArtistArt/retryAlbumArt's own doc comments.
export interface EntityGalleryConfig<Detail extends { artStatus: ArtStatus }, ArtSummary extends { artStatus: ArtStatus }> {
  fetchDetail: (id: number) => Promise<Detail>
  retryArt: (id: number) => Promise<ArtSummary>
  uploadImage: (id: number, file: File) => Promise<Detail>
  setPrimaryImage: (id: number, imageId: number) => Promise<Detail>
  deleteImage: (id: number, imageId: number, deleteFile: boolean) => Promise<Detail>
  deleteEntity: (id: number, deleteFiles: boolean) => Promise<void>
  afterRemove: string // route to navigate to once deleteEntity resolves
}

// useEntityGallery owns the fetch-on-mount, the gallery mutation handlers
// (retry/upload/set-primary/delete), and the remove-with-confirmation flow
// shared by ArtistDetailPage and AlbumDetailPage — previously ~65 lines
// of near-identical hooks/handlers duplicated per page.
export function useEntityGallery<Detail extends { artStatus: ArtStatus }, ArtSummary extends { artStatus: ArtStatus }>(
  id: number,
  cfg: EntityGalleryConfig<Detail, ArtSummary>,
) {
  const navigate = useNavigate()
  const [entity, setEntity] = useState<Detail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [removing, setRemoving] = useState(false)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setEntity(null)
    setLoadError(null)
    cfg
      .fetchDetail(id)
      .then((result) => {
        if (!cancelled) setEntity(result)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  async function retryArt() {
    const queued = await cfg.retryArt(id)
    setEntity((prev) => prev && ({ ...prev, ...queued } as Detail))
    const resolved = await pollForArtResolution(() => cfg.fetchDetail(id))
    setEntity((prev) => prev && ({ ...prev, ...resolved } as Detail))
  }

  async function uploadImage(file: File) {
    setEntity(await cfg.uploadImage(id, file))
  }

  async function setPrimaryImage(imageId: number) {
    setEntity(await cfg.setPrimaryImage(id, imageId))
  }

  async function deleteImage(imageId: number, deleteFile: boolean) {
    setEntity(await cfg.deleteImage(id, imageId, deleteFile))
  }

  function startRemove() {
    setRemoving(true)
  }

  function cancelRemove() {
    setRemoving(false)
    setRemoveError(null)
  }

  async function confirmRemove(deleteFiles: boolean) {
    setRemoveSubmitting(true)
    setRemoveError(null)
    try {
      await cfg.deleteEntity(id, deleteFiles)
      navigate(cfg.afterRemove)
    } catch (err) {
      setRemoveError(errorMessage(err))
      setRemoveSubmitting(false)
    }
  }

  return {
    entity,
    loadError,
    retryArt,
    uploadImage,
    setPrimaryImage,
    deleteImage,
    removing,
    removeSubmitting,
    removeError,
    startRemove,
    cancelRemove,
    confirmRemove,
  }
}
