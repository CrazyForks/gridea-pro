export interface ISetting {
  platform: 'github' | 'coding' | 'sftp' | 'gitee' | 'netlify' | 'vercel'
  domain: string
  repository: string
  branch: string
  username: string
  email: string
  tokenUsername: string
  token: string
  cname: string
  port: string
  server: string
  password: string
  privateKey: string
  remotePath: string
  netlifyAccessToken: string
  netlifySiteId: string
  platformConfigs?: Record<string, Record<string, any>>
  [index: string]: any
}

export interface ICommentSetting {
  showComment: boolean
  commentPlatform: string
  gitalkSetting?: any
  disqusSetting?: any
  [key: string]: any
}


