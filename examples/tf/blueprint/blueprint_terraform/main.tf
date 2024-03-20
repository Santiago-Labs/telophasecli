resource "aws_iam_role" "cross_account_role" {
  name = "CrossAccountRole"

  # This buildkite role can be assumed by ACCOUNT_iD
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::ACCOUNT_ID:role/BuildkiteRole"
        }
      },
    ]
  })
}

