-- FollowingFeed 本地开发文章演示数据（禁止用于生产环境）
-- 前置条件：数据库中已存在 id = 1 的用户（作者）。
-- MySQL 8.x；文章内容为原创摘要，末尾附公开参考来源。
SET NAMES utf8mb4;
START TRANSACTION;

-- TRUNCATE 同时清空数据并重置自增主键，避免重复 ID。
TRUNCATE TABLE `publish_articles`;
TRUNCATE TABLE `articles`;

-- 使用 REPLACE，即使客户端只执行了插入段，也会覆盖相同主键的旧记录。
REPLACE INTO `articles`
  (`id`, `title`, `content`, `author_id`, `status`, `created_at`, `updated_at`, `deleted_at`)
VALUES
(1, '从聊天机器人到智能代理：AI 正在进入企业工作流', '生成式 AI 的下一阶段不只是回答问题，而是理解目标、调用工具并完成多步骤任务。企业落地时应先选择边界清晰、可审计的流程，再通过权限控制、人工确认和效果指标逐步扩大自动化范围。\n\n参考：Microsoft AI Diffusion Report 2025 https://www.microsoft.com/en-us/corporate-responsibility/topics/AI-Economy-Institute/reports/Global-AI-Adoption-2025', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(2, '多智能体系统的价值：把复杂任务拆成可协作的模块', '多智能体架构适合研究、客服、运营和软件交付等需要分工协作的场景。真正的难点不是增加代理数量，而是定义角色边界、共享上下文、冲突处理和最终责任人。\n\n参考：Gartner Top Strategic Technology Trends 2026 https://www.gartner.com/en/articles/top-technology-trends-2026', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(3, '领域模型为何重新受到重视', '通用模型覆盖面广，但领域模型在专业术语、业务规则、合规要求和推理成本之间往往更容易取得平衡。团队可以从高质量领域数据、检索增强和小规模微调开始，而不是一开始就训练大模型。\n\n参考：Gartner Top Strategic Technology Trends 2026 https://www.gartner.com/en/articles/top-technology-trends-2026', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(4, 'AI 应用进入生产环境后，成本治理比模型参数更重要', '原型阶段关注效果，生产阶段还必须关注推理延迟、调用次数、缓存命中率和单位任务成本。把模型路由、提示词版本、输出评测和预算告警纳入平台能力，才能让 AI 项目从试验走向可持续运营。\n\n参考：Deloitte Tech Trends 2026 https://www.deloitte.com/nl/en/Industries/technology/perspectives/technology-trends.html', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(5, '软件工程的 AI 编程：效率提升与工程纪律要同时存在', 'AI 编程工具可以缩短样板代码和测试编写时间，但生成代码仍需要代码审查、自动化测试、依赖扫描和安全基线。最稳妥的用法是让 AI 加速开发者的反馈循环，而不是跳过设计和验证。\n\n参考：AI Index Report 2025 https://arxiv.org/abs/2504.07139', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(6, 'RAG 系统的核心竞争力：数据新鲜度与引用可追溯', '检索增强生成系统的回答质量取决于文档切分、元数据、召回策略和更新机制。面向真实业务，答案不仅要“像是正确”，还应能返回来源、时间和适用范围，让用户能够快速核验。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(7, 'AI 代理时代的身份与权限管理', '能够调用数据库、发送邮件或修改工单的代理，本质上是新的软件身份。权限应遵循最小授权原则，并区分读取、建议和执行三类能力；高风险操作应保留审批、审计日志和可撤销机制。\n\n参考：Google Cloud Cybersecurity Forecast 2026 https://cloud.google.com/security/resources/cybersecurity-forecast', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(8, 'Shadow AI 与 Shadow Agent：企业治理的新盲区', '员工私自使用 AI 工具已经带来数据泄露和合规风险，而可自主行动的 Shadow Agent 还可能扩大影响范围。企业需要建立工具清单、数据分级、代理注册和行为监控，让安全治理跟上使用方式的变化。\n\n参考：Google Cloud Cybersecurity Forecast 2026 https://cloud.google.com/security/resources/cybersecurity-forecast', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(9, '网络安全从事后响应转向主动防御', '攻击者正在使用自动化和 AI 提升侦察、钓鱼与漏洞利用效率。防守方应把攻击面管理、威胁情报、身份保护和持续暴露面评估结合起来，在事件发生前减少可利用路径。\n\n参考：ENISA Threat Landscape 2025 https://www.enisa.europa.eu/publications/enisa-threat-landscape-2025', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(10, 'API 安全成为数字业务的基础设施问题', '当业务通过 API 连接用户、设备和智能代理时，越权访问、敏感数据暴露和接口滥用会直接影响业务。API 设计应从认证授权、输入校验、速率限制、版本管理和可观测性一体化考虑。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(11, '云原生架构的下一课：可观测性必须服务于决策', '日志、指标和链路追踪不应只是事后排障工具。把它们与用户体验、业务转化、资源成本和发布变更关联起来，团队才能回答“哪里变慢了”之外的关键问题：影响了谁、损失了什么、是否值得修复。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(12, '边缘计算与端侧 AI：低延迟之外还要考虑数据边界', '端侧推理可以减少网络往返、降低云端成本，并让部分敏感数据留在本地。落地时需要平衡设备算力、模型更新、离线能力、安全启动和隐私保护，不能只比较单次响应速度。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(13, '主权云与区域化部署：合规正在影响云架构选择', '数据驻留、供应链风险和监管要求，使企业开始重新评估跨区域云服务依赖。区域化部署并不等于简单搬迁，还需要在身份、密钥、备份、灾备和运维能力上建立独立性。\n\n参考：Gartner Top Strategic Technology Trends 2026 https://www.gartner.com/en/articles/top-technology-trends-2026', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(14, '从平台工程到 AI 工程平台', '成熟的 AI 平台应覆盖数据准备、提示词与模型管理、评测、部署、监控和成本核算。平台团队的目标不是替业务团队选择唯一模型，而是提供安全、可复用且能快速迭代的交付路径。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(15, '数据质量决定 AI 项目的上限', '模型能力提升并不能自动修复重复、过期、缺失或权限不清的数据。建立数据责任人、质量规则、血缘关系和反馈闭环，通常比继续堆叠更多模型参数更能改善业务结果。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(16, '数字信任的重建：内容真实性需要技术与制度协同', '合成内容越来越普遍，平台和企业需要在发布、传播和消费环节提供更清晰的来源标识与验证能力。水印、签名、内容凭证可以提供证据，但最终仍需要媒体素养、审核流程和责任机制配合。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(17, '隐私计算的现实路径：先解决可量化的协作问题', '联邦学习、可信执行环境和安全多方计算为跨机构数据协作提供了不同选择。选型时应从参与方信任关系、性能预算、数据敏感度和审计要求出发，优先解决一个边界明确的业务问题。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(18, '绿色计算不只是节能：把碳排放纳入工程指标', 'AI 和云服务规模扩大后，算力、制冷和数据传输都会带来环境成本。通过模型压缩、弹性调度、合理选区和碳感知工作负载管理，工程团队可以在性能、费用和可持续性之间找到更好的平衡。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(19, '互联网产品的增长逻辑正在从流量转向信任', '在内容泛滥和获客成本上升的环境中，产品的长期竞争力来自可靠体验、透明规则和持续价值。推荐系统、广告系统和 AI 功能都应把用户控制权、解释能力与隐私保护作为产品设计的一部分。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(20, '开发者体验是技术组织的生产力基础设施', '构建速度、测试反馈、文档可发现性和发布安全性，会持续影响团队交付能力。内部开发者平台应减少重复劳动，同时保留清晰的责任边界，让“更快发布”和“更可靠运行”成为同一个目标。', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0),
(21, '技术决策回归问题本身：避免被热点牵着走', '面对 AI、云和安全等快速变化的技术热点，最有效的路线仍是从用户问题、业务目标和约束条件出发。用可验证的试点、明确的成功指标和退出机制控制不确定性，才能把创新变成稳定能力。\n\n参考：Deloitte Tech Trends 2026 https://www.deloitte.com/nl/en/Industries/technology/perspectives/technology-trends.html', 1, 2, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000, 0);

REPLACE INTO `publish_articles`
  (`id`, `title`, `content`, `author_id`, `status`, `created_at`, `updated_at`, `deleted_at`)
SELECT `id`, `title`, `content`, `author_id`, `status`, `created_at`, `updated_at`, `deleted_at`
FROM `articles`;

COMMIT;

-- 如果应用启用了 Redis 文章首页缓存，请执行：
-- DEL article:first_page:1;
