<template>
  <div class="users-page">
    <el-card class="page-heading" shadow="never">
      <div>
        <div class="eyebrow">用户中心</div>
        <h1>用户管理</h1>
        <p>管理小程序用户资料与账号状态</p>
      </div>
      <el-button icon="el-icon-refresh" @click="loadUsers">刷新</el-button>
    </el-card>

    <el-row :gutter="16" class="stat-row">
      <el-col v-for="card in statCards" :key="card.key" :xs="12" :sm="6">
        <el-card shadow="hover" class="stat-card">
          <span>{{ card.label }}</span>
          <strong>{{ stats[card.key] }}</strong>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never">
      <el-form :inline="true" class="filter-form" @submit.native.prevent="search">
        <el-form-item>
          <el-input v-model="filters.query" clearable placeholder="搜索用户 ID、账号或昵称" @keyup.enter.native="search" />
        </el-form-item>
        <el-form-item>
          <el-select v-model="filters.status" placeholder="账号状态" @change="search">
            <el-option label="全部状态" value="all" />
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select v-model="filters.platform" placeholder="登录平台" @change="search">
            <el-option label="全部平台" value="all" />
            <el-option label="微信" value="wechat" />
            <el-option label="TapTap" value="taptap" />
            <el-option label="账号" value="password" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" icon="el-icon-search" @click="search">查询</el-button>
          <el-button @click="resetFilters">重置</el-button>
        </el-form-item>
        <el-form-item class="bulk-action">
          <el-button type="danger" :disabled="selected.length === 0" @click="batchDisable">批量禁用</el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="users" stripe @selection-change="selected = $event" @row-click="openDetail">
        <el-table-column type="selection" width="52" />
        <el-table-column prop="id" label="用户 ID" width="100" />
        <el-table-column label="用户" min-width="220">
          <template slot-scope="scope">
            <div class="identity-cell">
              <span class="avatar-fallback">{{ initials(scope.row.nickname) }}</span>
              <span>
                <strong>{{ scope.row.nickname }}</strong>
                <small>{{ scope.row.username }}</small>
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="platform" label="平台" width="100" />
        <el-table-column label="状态" width="110">
          <template slot-scope="scope">
            <el-tag :type="scope.row.status === 'disabled' ? 'danger' : 'success'" size="small">
              {{ scope.row.status === 'disabled' ? '已禁用' : '正常' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="注册时间" min-width="170">
          <template slot-scope="scope">{{ formatDate(scope.row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template slot-scope="scope">
            <el-button type="text" @click.stop="openDetail(scope.row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        background
        layout="total, sizes, prev, pager, next"
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[20, 50, 100]"
        :total="total"
        class="pagination"
        @current-change="changePage"
        @size-change="changePageSize"
      />
    </el-card>

    <el-drawer title="用户详情" :visible.sync="drawerVisible" size="430px" @close="detail = null">
      <div v-if="detail" v-loading="detailLoading" class="detail-panel">
        <div class="detail-identity">
          <span class="avatar-fallback avatar-fallback--large">{{ initials(detail.nickname) }}</span>
          <div>
            <h2>{{ detail.nickname }}</h2>
            <p>{{ detail.username }} · ID {{ detail.id }}</p>
          </div>
        </div>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="平台">{{ detail.platform }}</el-descriptions-item>
          <el-descriptions-item label="账号状态">{{ detail.status === 'disabled' ? '已禁用' : '正常' }}</el-descriptions-item>
          <el-descriptions-item label="昵称审核">{{ moderationLabel(detail.nicknameModerationStatus) }}</el-descriptions-item>
          <el-descriptions-item label="头像审核">{{ moderationLabel(detail.avatarModerationStatus) }}</el-descriptions-item>
          <el-descriptions-item label="注册时间">{{ formatDate(detail.createdAt) }}</el-descriptions-item>
          <el-descriptions-item label="最近活跃">{{ formatDate(detail.lastActiveAt) }}</el-descriptions-item>
        </el-descriptions>
        <el-alert v-if="detail.nicknameModerationStatus === 'rejected'" title="昵称未通过审核，当前展示安全昵称" type="warning" :closable="false" show-icon />
        <el-alert v-if="detail.avatarModerationStatus === 'rejected'" title="头像未通过审核，当前未展示原头像" type="warning" :closable="false" show-icon />
        <el-button class="status-button" :type="detail.status === 'disabled' ? 'success' : 'danger'" @click="toggleStatus(detail)">
          {{ detail.status === 'disabled' ? '启用账号' : '禁用账号' }}
        </el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script>
import { listUsers, getUser, disableUser, enableUser, AdminApiError } from '@/api/admin'

const mapUser = raw => ({
  id: String(raw.id || ''),
  username: raw.username || '--',
  nickname: raw.nickname || '算术玩家',
  platform: { wechat: '微信', taptap: 'TapTap', password: '账号' }[raw.platform] || '账号',
  status: Number(raw.status) === 0 || raw.status === 'disabled' ? 'disabled' : 'active',
  createdAt: raw.created_at || raw.createdAt || '',
  lastActiveAt: raw.last_active_at || raw.updated_at || raw.updatedAt || '',
  nicknameModerationStatus: raw.nickname_moderation_status,
  avatarModerationStatus: raw.avatar_moderation_status
})

export default {
  name: 'UserManagement',
  data() {
    return {
      filters: { query: '', status: 'all', platform: 'all' },
      users: [],
      selected: [],
      stats: { total: 0, new_today: 0, active: 0, disabled: 0 },
      page: 1,
      pageSize: 20,
      total: 0,
      loading: false,
      drawerVisible: false,
      detailLoading: false,
      detail: null,
      statCards: [
        { key: 'total', label: '用户总数' },
        { key: 'new_today', label: '今日新增' },
        { key: 'active', label: '活跃用户' },
        { key: 'disabled', label: '已禁用用户' }
      ]
    }
  },
  created() { this.loadUsers() },
  methods: {
    async loadUsers() {
      this.loading = true
      try {
        const data = await listUsers({ ...this.filters, page: this.page, pageSize: this.pageSize })
        this.users = (data.items || []).map(mapUser)
        this.stats = data.stats || this.stats
        this.total = Number(data.total) || 0
        this.page = Number(data.page) || this.page
        this.pageSize = Number(data.page_size) || this.pageSize
      } catch (error) {
        this.handleError(error)
      } finally {
        this.loading = false
      }
    },
    search() {
      this.page = 1
      this.selected = []
      this.loadUsers()
    },
    resetFilters() {
      this.filters = { query: '', status: 'all', platform: 'all' }
      this.search()
    },
    changePage(page) { this.page = page; this.loadUsers() },
    changePageSize(size) { this.pageSize = size; this.page = 1; this.loadUsers() },
    async openDetail(row) {
      this.drawerVisible = true
      this.detailLoading = true
      try {
        this.detail = mapUser(await getUser(row.id))
      } catch (error) {
        this.handleError(error)
        this.drawerVisible = false
      } finally {
        this.detailLoading = false
      }
    },
    async toggleStatus(user) {
      const disabled = user.status !== 'disabled'
      try {
        await this.$confirm(`确定要${disabled ? '禁用' : '启用'}账号 ${user.id} 吗？`, '确认操作', { type: 'warning' })
        if (disabled) await disableUser(user.id)
        else await enableUser(user.id)
        this.$message.success(disabled ? '账号已禁用' : '账号已启用')
        await this.loadUsers()
        this.detail = mapUser(await getUser(user.id))
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') this.handleError(error)
      }
    },
    async batchDisable() {
      try {
        await this.$confirm(`确定要禁用选中的 ${this.selected.length} 个账号吗？`, '确认操作', { type: 'warning' })
        const failures = []
        for (const user of this.selected) {
          try { await disableUser(user.id) } catch (error) { failures.push(`${user.id}: ${error.message}`) }
        }
        this.$message[failures.length ? 'warning' : 'success'](failures.length ? `部分操作失败：${failures.join('；')}` : '选中账号已禁用')
        this.selected = []
        await this.loadUsers()
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') this.handleError(error)
      }
    },
    handleError(error) {
      if (error instanceof AdminApiError && error.isAuthError) {
        this.$store.dispatch('user/resetToken')
        this.$router.push(`/login?redirect=${this.$route.fullPath}`)
        return
      }
      this.$message.error(error.message || '请求失败，请稍后重试')
    },
    initials(value) {
      const text = String(value || '').trim()
      return text.length <= 2 ? text || '--' : `${text.slice(0, 1)}${text.slice(-1)}`
    },
    formatDate(value) {
      if (!value) return '--'
      const date = new Date(value)
      return Number.isNaN(date.getTime()) ? '--' : date.toLocaleString('zh-CN', { hour12: false })
    },
    moderationLabel(status) {
      return { approved: '已通过', passed: '已通过', pending: '审核中', rejected: '未通过', unreviewed: '待审核' }[status] || '未提供'
    }
  }
}
</script>

<style lang="scss" scoped>
.users-page { padding: 24px; }
.page-heading { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-heading h1 { margin: 4px 0; font-size: 26px; color: #1f2d3d; }
.page-heading p { margin: 0; color: #7b8794; }
.eyebrow { color: #409eff; font-size: 12px; letter-spacing: .08em; }
.stat-row { margin-bottom: 16px; }
.stat-card { min-height: 86px; display: flex; flex-direction: column; justify-content: space-between; }
.stat-card span { color: #7b8794; font-size: 13px; }
.stat-card strong { color: #1f2d3d; font-size: 28px; line-height: 1.2; }
.filter-form { margin-bottom: -18px; }
.bulk-action { float: right; }
.identity-cell, .detail-identity { display: flex; align-items: center; gap: 12px; }
.identity-cell strong, .identity-cell small { display: block; }
.identity-cell small { margin-top: 4px; color: #98a2b3; }
.avatar-fallback { width: 34px; height: 34px; display: inline-flex; align-items: center; justify-content: center; color: #fff; background: linear-gradient(135deg, #409eff, #67c23a); border-radius: 50%; font-size: 12px; }
.avatar-fallback--large { width: 56px; height: 56px; font-size: 18px; }
.pagination { margin-top: 20px; text-align: right; }
.detail-panel { padding: 4px 20px 20px; }
.detail-identity { margin-bottom: 20px; }
.detail-identity h2 { margin: 0 0 6px; font-size: 20px; }
.detail-identity p { margin: 0; color: #98a2b3; }
.detail-panel .el-alert { margin-top: 16px; }
.status-button { width: 100%; margin-top: 22px; }
@media (max-width: 768px) {
  .users-page { padding: 12px; }
  .page-heading { align-items: flex-start; }
  .bulk-action { float: none; }
}
</style>
