data "aws_iam_policy_document" "restrict_regions" {
  statement {
    sid       = "RegionRestriction"
    effect    = "Deny"
    actions   = ["*"]
    resources = ["*"]

    condition {
      test     = "StringNotEquals"
      variable = "aws:RequestedRegion"

      values = [
        "eu-west-1"
      ]
    }
  }
}

resource "aws_organizations_policy" "restrict_regions" {
  name        = "restrict_regions"
  description = "Deny all regions except EU West 1."
  content     = data.aws_iam_policy_document.restrict_regions.json
}

resource "aws_organizations_policy_attachment" "restrict_regions_on_account" {
  policy_id = aws_organizations_policy.restrict_regions.id
  target_id = telophase.account_id
}
