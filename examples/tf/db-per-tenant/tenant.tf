resource "neon_project" "tenant" {
  name      = telophase.account_name
  region_id = "aws-us-east-1"
  branch = {
    endpoint = {
      suspend_timeout = 300
    }
  }
}

resource "neon_role" "tenant" {
  name       = "${telophase.account_name}_user"
  branch_id  = neon_project.tenant.branch.id
  project_id = neon_project.tenant.id
}

resource "neon_database" "tenant" {
  name       = "${telophase.account_name}db"
  owner_name = neon_role.tenant.name
  branch_id  = neon_project.tenant.branch.id
  project_id = neon_project.tenant.id
}

