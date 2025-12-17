export const useNavigate = () => {
  const navigate = (url: string) => {
    window.location.href = url
  }

  return navigate
}

